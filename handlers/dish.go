package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/uptrace/bun"

	"yy-kitchen-logic/models"
)

// RegisterAdminDishOps registers the full dish CRUD on the admin API.
func RegisterAdminDishOps(api huma.API, db *bun.DB) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-list-dishes",
		Method:      http.MethodGet,
		Path:        "/api/admin/v1/dishes",
		Summary:     "菜品列表",
		Description: "支持按类别过滤及分页,返回分页信封。",
		Tags:        []string{"后台-菜品"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, listDishes(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-get-dish",
		Method:      http.MethodGet,
		Path:        "/api/admin/v1/dishes/{id}",
		Summary:     "菜品详情",
		Description: "根据菜品 ID 获取详情。",
		Tags:        []string{"后台-菜品"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, getDish(db))

	huma.Register(api, huma.Operation{
		OperationID:   "admin-create-dish",
		Method:        http.MethodPost,
		Path:          "/api/admin/v1/dishes/create",
		Summary:       "新增菜品",
		Description:   "创建一条菜品记录。",
		Tags:          []string{"后台-菜品"},
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: http.StatusCreated,
	}, createDish(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-update-dish",
		Method:      http.MethodPost,
		Path:        "/api/admin/v1/dishes/update",
		Summary:     "更新菜品",
		Description: "字段均为可选,仅对传入字段做更新。id 必填。",
		Tags:        []string{"后台-菜品"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, updateDish(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-delete-dish",
		Method:      http.MethodPost,
		Path:        "/api/admin/v1/dishes/delete",
		Summary:     "删除菜品",
		Description: "软删除指定菜品(deleted_at 会被填充)。id 在 body 中传入。",
		Tags:        []string{"后台-菜品"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, deleteDish(db))
}

// RegisterAppDishOps registers dish query operations available to the APP.
func RegisterAppDishOps(api huma.API, db *bun.DB) {
	huma.Register(api, huma.Operation{
		OperationID: "app-list-dishes",
		Method:      http.MethodGet,
		Path:        "/api/app/v1/dishes",
		Summary:     "菜品列表",
		Description: "支持按类别过滤及分页,返回分页信封。",
		Tags:        []string{"APP-菜品"},
	}, listDishes(db))

	huma.Register(api, huma.Operation{
		OperationID: "app-get-dish",
		Method:      http.MethodGet,
		Path:        "/api/app/v1/dishes/{id}",
		Summary:     "菜品详情",
		Description: "根据菜品 ID 获取详情。",
		Tags:        []string{"APP-菜品"},
	}, getDish(db))
}

// ---- List

type ListDishesInput struct {
	Category int `query:"category" minimum:"1" maximum:"4" doc:"菜品类别:1=主食 2=主菜 3=饮料 4=甜点。留空表示不过滤。"`
	Page     int `query:"page" minimum:"1" default:"1" doc:"页码,从 1 开始"`
	PageSize int `query:"page_size" minimum:"1" maximum:"100" default:"20" doc:"每页数量,最大 100"`
}

type ListDishesOutput struct {
	Body struct {
		Items    []models.Dish `json:"items"`
		Total    int           `json:"total"`
		Page     int           `json:"page"`
		PageSize int           `json:"page_size"`
	}
}

func listDishes(db *bun.DB) func(context.Context, *ListDishesInput) (*ListDishesOutput, error) {
	return func(ctx context.Context, in *ListDishesInput) (*ListDishesOutput, error) {
		page := in.Page
		if page < 1 {
			page = 1
		}
		pageSize := in.PageSize
		if pageSize < 1 {
			pageSize = 20
		}
		if pageSize > 100 {
			pageSize = 100
		}

		var dishes []models.Dish
		q := db.NewSelect().Model(&dishes)
		if in.Category != 0 {
			if !models.DishCategory(in.Category).IsValid() {
				return nil, huma.Error400BadRequest("invalid category")
			}
			q = q.Where("category = ?", in.Category)
		}

		total, err := q.Count(ctx)
		if err != nil {
			return nil, err
		}
		if err := q.Order("id ASC").Limit(pageSize).Offset((page - 1) * pageSize).Scan(ctx); err != nil {
			return nil, err
		}

		out := &ListDishesOutput{}
		out.Body.Items = dishes
		out.Body.Total = total
		out.Body.Page = page
		out.Body.PageSize = pageSize
		return out, nil
	}
}

// ---- Get / Create / Update / Delete

type DishIDInput struct {
	ID int64 `path:"id" doc:"菜品 ID"`
}

type DishOutput struct {
	Body models.Dish
}

func getDish(db *bun.DB) func(context.Context, *DishIDInput) (*DishOutput, error) {
	return func(ctx context.Context, in *DishIDInput) (*DishOutput, error) {
		var dish models.Dish
		err := db.NewSelect().Model(&dish).Where("id = ?", in.ID).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("dish not found")
		}
		if err != nil {
			return nil, err
		}
		return &DishOutput{Body: dish}, nil
	}
}

type CreateDishInput struct {
	Body struct {
		Name        string              `json:"name" required:"true" example:"宫保鸡丁" doc:"菜品名称"`
		Description string              `json:"description" required:"true" example:"经典川菜,微辣" doc:"菜品描述"`
		Icon        string              `json:"icon" required:"true" example:"https://cdn.example.com/dishes/kungpao.png" doc:"图标 URL"`
		Category    models.DishCategory `json:"category" required:"true" minimum:"1" maximum:"4" doc:"菜品类别:1=主食 2=主菜 3=饮料 4=甜点"`
	}
}

func createDish(db *bun.DB) func(context.Context, *CreateDishInput) (*DishOutput, error) {
	return func(ctx context.Context, in *CreateDishInput) (*DishOutput, error) {
		if !in.Body.Category.IsValid() {
			return nil, huma.Error400BadRequest("invalid category")
		}
		now := time.Now()
		dish := models.Dish{
			Name:        in.Body.Name,
			Description: in.Body.Description,
			Icon:        in.Body.Icon,
			Category:    in.Body.Category,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if _, err := db.NewInsert().Model(&dish).Exec(ctx); err != nil {
			return nil, err
		}
		return &DishOutput{Body: dish}, nil
	}
}

type UpdateDishInput struct {
	Body struct {
		ID          int64                `json:"id" required:"true" doc:"菜品 ID"`
		Name        *string              `json:"name,omitempty" doc:"菜品名称"`
		Description *string              `json:"description,omitempty" doc:"菜品描述"`
		Icon        *string              `json:"icon,omitempty" doc:"图标 URL"`
		Category    *models.DishCategory `json:"category,omitempty" minimum:"1" maximum:"4" doc:"菜品类别:1=主食 2=主菜 3=饮料 4=甜点"`
	}
}

func updateDish(db *bun.DB) func(context.Context, *UpdateDishInput) (*DishOutput, error) {
	return func(ctx context.Context, in *UpdateDishInput) (*DishOutput, error) {
		var dish models.Dish
		err := db.NewSelect().Model(&dish).Where("id = ?", in.Body.ID).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("dish not found")
		}
		if err != nil {
			return nil, err
		}
		if in.Body.Name != nil {
			dish.Name = *in.Body.Name
		}
		if in.Body.Description != nil {
			dish.Description = *in.Body.Description
		}
		if in.Body.Icon != nil {
			dish.Icon = *in.Body.Icon
		}
		if in.Body.Category != nil {
			if !in.Body.Category.IsValid() {
				return nil, huma.Error400BadRequest("invalid category")
			}
			dish.Category = *in.Body.Category
		}
		dish.UpdatedAt = time.Now()
		if _, err := db.NewUpdate().Model(&dish).WherePK().Exec(ctx); err != nil {
			return nil, err
		}
		return &DishOutput{Body: dish}, nil
	}
}

type DeleteDishInput struct {
	Body struct {
		ID int64 `json:"id" required:"true" doc:"菜品 ID"`
	}
}

type DeleteDishOutput struct {
	Body struct {
		Deleted int64 `json:"deleted"`
	}
}

func deleteDish(db *bun.DB) func(context.Context, *DeleteDishInput) (*DeleteDishOutput, error) {
	return func(ctx context.Context, in *DeleteDishInput) (*DeleteDishOutput, error) {
		if _, err := db.NewDelete().Model((*models.Dish)(nil)).Where("id = ?", in.Body.ID).Exec(ctx); err != nil {
			return nil, err
		}
		out := &DeleteDishOutput{}
		out.Body.Deleted = in.Body.ID
		return out, nil
	}
}
