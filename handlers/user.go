package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"

	"yy-kitchen-logic/middleware"
	"yy-kitchen-logic/models"
)

// RegisterAdminUserOps registers all user CRUD operations on the admin API.
func RegisterAdminUserOps(api huma.API, db *bun.DB) {
	huma.Register(api, huma.Operation{
		OperationID:   "admin-create-user",
		Method:        http.MethodPost,
		Path:          "/api/admin/v1/users/create",
		Summary:       "新增用户",
		Description:   "由管理后台创建用户。密码在写入前会用 bcrypt 加盐哈希。",
		Tags:          []string{"后台-用户"},
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: http.StatusCreated,
	}, createUser(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-list-users",
		Method:      http.MethodGet,
		Path:        "/api/admin/v1/users",
		Summary:     "用户列表",
		Description: "返回全部用户。",
		Tags:        []string{"后台-用户"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, listUsers(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-get-user",
		Method:      http.MethodGet,
		Path:        "/api/admin/v1/users/{id}",
		Summary:     "用户详情",
		Description: "根据用户 ID 获取用户信息。",
		Tags:        []string{"后台-用户"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, getUser(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-update-user",
		Method:      http.MethodPost,
		Path:        "/api/admin/v1/users/update",
		Summary:     "更新用户",
		Description: "字段均为可选,仅对传入字段做更新。密码会被重新加盐哈希。id 必填。",
		Tags:        []string{"后台-用户"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, updateUser(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-delete-user",
		Method:      http.MethodPost,
		Path:        "/api/admin/v1/users/delete",
		Summary:     "删除用户",
		Description: "软删除指定用户。id 在 body 中传入。",
		Tags:        []string{"后台-用户"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, deleteUser(db))
}

// RegisterAppUserOps registers user-facing endpoints available in the APP.
func RegisterAppUserOps(api huma.API, db *bun.DB) {
	huma.Register(api, huma.Operation{
		OperationID: "app-me",
		Method:      http.MethodGet,
		Path:        "/api/app/v1/me",
		Summary:     "获取当前登录用户",
		Description: "根据 JWT 中的用户 ID 返回当前登录者的信息。",
		Tags:        []string{"APP-用户"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, me(db))

	huma.Register(api, huma.Operation{
		OperationID: "app-update-profile",
		Method:      http.MethodPost,
		Path:        "/api/app/v1/me/update",
		Summary:     "修改昵称/头像",
		Description: "当前登录用户可修改自己的昵称和头像地址。字段均为可选,仅对传入字段做更新。",
		Tags:        []string{"APP-用户"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, updateProfile(db))
}

// ---- shared

type UserIDInput struct {
	ID int64 `path:"id" doc:"用户 ID"`
}

type UserOutput struct {
	Body models.User
}

// ---- Create

type CreateUserInput struct {
	Body struct {
		Name     string `json:"name" required:"true" example:"lemon" doc:"用户名"`
		Account  string `json:"account" required:"true" pattern:"^1[3-9][0-9]{9}$" example:"13800138000" doc:"账号(手机号,中国大陆 11 位)"`
		Password string `json:"password" required:"true" minLength:"6" example:"strong-password" doc:"密码,至少 6 位"`
	}
}

func createUser(db *bun.DB) func(context.Context, *CreateUserInput) (*UserOutput, error) {
	return func(ctx context.Context, in *CreateUserInput) (*UserOutput, error) {
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Body.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		now := time.Now()

		// 检查账号是否已存在(包括软删除记录)
		var existing models.User
		err = db.NewSelect().Model(&existing).WhereAllWithDeleted().Where("account = ?", in.Body.Account).Scan(ctx)
		if err == nil {
			if existing.DeletedAt == nil {
				return nil, huma.Error400BadRequest("该手机号已注册")
			}
			// 软删除用户重新激活
			existing.Name = in.Body.Name
			existing.Password = string(hash)
			existing.Avatar = ""
			existing.DeletedAt = nil
			existing.UpdatedAt = now
			if _, err := db.NewUpdate().Model(&existing).WherePK().WhereAllWithDeleted().Exec(ctx); err != nil {
				return nil, err
			}
			return &UserOutput{Body: existing}, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}

		user := models.User{
			Name:      in.Body.Name,
			Account:   in.Body.Account,
			Password:  string(hash),
			CreatedAt: now,
			UpdatedAt: now,
		}
		if _, err := db.NewInsert().Model(&user).Exec(ctx); err != nil {
			if isDuplicateKeyErr(err) {
				return nil, huma.Error400BadRequest("该手机号已注册")
			}
			return nil, err
		}
		return &UserOutput{Body: user}, nil
	}
}

// ---- List

type ListUsersOutput struct {
	Body struct {
		Items []models.User `json:"items"`
	}
}

func listUsers(db *bun.DB) func(context.Context, *struct{}) (*ListUsersOutput, error) {
	return func(ctx context.Context, _ *struct{}) (*ListUsersOutput, error) {
		var users []models.User
		if err := db.NewSelect().Model(&users).Scan(ctx); err != nil {
			return nil, err
		}
		out := &ListUsersOutput{}
		out.Body.Items = users
		return out, nil
	}
}

// ---- Get

func getUser(db *bun.DB) func(context.Context, *UserIDInput) (*UserOutput, error) {
	return func(ctx context.Context, in *UserIDInput) (*UserOutput, error) {
		var user models.User
		err := db.NewSelect().Model(&user).Where("id = ?", in.ID).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("user not found")
		}
		if err != nil {
			return nil, err
		}
		return &UserOutput{Body: user}, nil
	}
}

// ---- Update

type UpdateUserInput struct {
	Body struct {
		ID       int64   `json:"id" required:"true" doc:"用户 ID"`
		Name     *string `json:"name,omitempty" doc:"用户名"`
		Account  *string `json:"account,omitempty" pattern:"^1[3-9][0-9]{9}$" doc:"账号(手机号)"`
		Password *string `json:"password,omitempty" minLength:"6" doc:"新密码,至少 6 位"`
	}
}

func updateUser(db *bun.DB) func(context.Context, *UpdateUserInput) (*UserOutput, error) {
	return func(ctx context.Context, in *UpdateUserInput) (*UserOutput, error) {
		var user models.User
		err := db.NewSelect().Model(&user).Where("id = ?", in.Body.ID).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("user not found")
		}
		if err != nil {
			return nil, err
		}
		if in.Body.Name != nil {
			user.Name = *in.Body.Name
		}
		if in.Body.Account != nil && *in.Body.Account != user.Account {
			var existing models.User
			err := db.NewSelect().Model(&existing).WhereAllWithDeleted().Where("account = ? AND id != ?", *in.Body.Account, user.ID).Scan(ctx)
			if err == nil {
				return nil, huma.Error400BadRequest("该手机号已被其他用户使用")
			} else if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			user.Account = *in.Body.Account
		}
		if in.Body.Password != nil {
			hash, err := bcrypt.GenerateFromPassword([]byte(*in.Body.Password), bcrypt.DefaultCost)
			if err != nil {
				return nil, err
			}
			user.Password = string(hash)
		}
		user.UpdatedAt = time.Now()
		if _, err := db.NewUpdate().Model(&user).WherePK().Exec(ctx); err != nil {
			if isDuplicateKeyErr(err) {
				return nil, huma.Error400BadRequest("该手机号已被其他用户使用")
			}
			return nil, err
		}
		return &UserOutput{Body: user}, nil
	}
}

// ---- Delete

type DeleteUserInput struct {
	Body struct {
		ID int64 `json:"id" required:"true" doc:"用户 ID"`
	}
}

type DeleteUserOutput struct {
	Body struct {
		Deleted int64 `json:"deleted"`
	}
}

func deleteUser(db *bun.DB) func(context.Context, *DeleteUserInput) (*DeleteUserOutput, error) {
	return func(ctx context.Context, in *DeleteUserInput) (*DeleteUserOutput, error) {
		if _, err := db.NewDelete().Model((*models.User)(nil)).Where("id = ?", in.Body.ID).Exec(ctx); err != nil {
			return nil, err
		}
		out := &DeleteUserOutput{}
		out.Body.Deleted = in.Body.ID
		return out, nil
	}
}

// ---- Me

func me(db *bun.DB) func(context.Context, *struct{}) (*UserOutput, error) {
	return func(ctx context.Context, _ *struct{}) (*UserOutput, error) {
		uid, ok := ctx.Value(middleware.ContextUserIDKey).(uint)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}
		var user models.User
		err := db.NewSelect().Model(&user).Where("id = ?", int64(uid)).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("user not found")
		}
		if err != nil {
			return nil, err
		}
		return &UserOutput{Body: user}, nil
	}
}

// ---- App: Update Profile (name / avatar)

type UpdateProfileInput struct {
	Body struct {
		Name   *string `json:"name,omitempty" minLength:"1" maxLength:"32" doc:"昵称"`
		Avatar *string `json:"avatar,omitempty" maxLength:"512" doc:"头像地址 (通常来自 /api/admin/v1/upload 返回的 url)"`
	}
}

func updateProfile(db *bun.DB) func(context.Context, *UpdateProfileInput) (*UserOutput, error) {
	return func(ctx context.Context, in *UpdateProfileInput) (*UserOutput, error) {
		uid, ok := ctx.Value(middleware.ContextUserIDKey).(uint)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}
		if in.Body.Name == nil && in.Body.Avatar == nil {
			return nil, huma.Error400BadRequest("name 与 avatar 至少传一个")
		}

		var user models.User
		err := db.NewSelect().Model(&user).Where("id = ?", int64(uid)).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("user not found")
		}
		if err != nil {
			return nil, err
		}
		if in.Body.Name != nil {
			user.Name = *in.Body.Name
		}
		if in.Body.Avatar != nil {
			user.Avatar = *in.Body.Avatar
		}
		user.UpdatedAt = time.Now()
		if _, err := db.NewUpdate().Model(&user).WherePK().Exec(ctx); err != nil {
			return nil, err
		}
		return &UserOutput{Body: user}, nil
	}
}

func isDuplicateKeyErr(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return true
	}
	return false
}
