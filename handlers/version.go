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

// RegisterAdminVersionOps registers version CRUD on the admin API.
func RegisterAdminVersionOps(api huma.API, db *bun.DB) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-list-app-versions",
		Method:      http.MethodGet,
		Path:        "/api/admin/v1/version",
		Summary:     "版本列表",
		Description: "按 version_code 降序返回历史版本,支持分页。",
		Tags:        []string{"后台-版本"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, listVersions(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-get-app-version",
		Method:      http.MethodGet,
		Path:        "/api/admin/v1/version/{id}",
		Summary:     "版本详情",
		Description: "根据版本记录 ID 获取详情。",
		Tags:        []string{"后台-版本"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, getVersion(db))

	huma.Register(api, huma.Operation{
		OperationID:   "admin-create-app-version",
		Method:        http.MethodPost,
		Path:          "/api/admin/v1/version/create",
		Summary:       "发布新版本",
		Description:   "登记一个新的 APK 版本。version_code 唯一且需大于所有历史版本。",
		Tags:          []string{"后台-版本"},
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: http.StatusCreated,
	}, createVersion(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-update-app-version",
		Method:      http.MethodPost,
		Path:        "/api/admin/v1/version/update",
		Summary:     "更新版本",
		Description: "字段均为可选,仅对传入字段做更新。id 必填。version_code 不可修改。",
		Tags:        []string{"后台-版本"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, updateVersion(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-delete-app-version",
		Method:      http.MethodPost,
		Path:        "/api/admin/v1/version/delete",
		Summary:     "删除版本",
		Description: "软删除指定版本记录。id 在 body 中传入。",
		Tags:        []string{"后台-版本"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, deleteVersion(db))
}

// RegisterAppVersionOps registers the version check endpoint on the APP API.
func RegisterAppVersionOps(api huma.API, db *bun.DB) {
	huma.Register(api, huma.Operation{
		OperationID: "app-check-app-version",
		Method:      http.MethodPost,
		Path:        "/api/app/v1/version/check",
		Summary:     "APK 版本检测",
		Description: "客户端上报当前 version_code,服务端返回是否需要升级、是否强制升级以及最新版本信息。",
		Tags:        []string{"APP-版本"},
	}, checkVersion(db))
}

// ---- Check

type CheckVersionInput struct {
	Body struct {
		VersionCode int64  `json:"version_code" required:"true" minimum:"0" example:"10" doc:"当前 APK 版本号"`
		VersionName string `json:"version_name,omitempty" example:"1.1.0" doc:"当前版本名(可选)"`
	}
}

type CheckVersionOutput struct {
	Body struct {
		HasUpdate   bool               `json:"has_update" doc:"是否有新版本"`
		ForceUpdate bool               `json:"force_update" doc:"当前版本是否低于最小支持版本,需强制升级"`
		Latest      *models.AppVersion `json:"latest,omitempty" doc:"最新版本信息,若尚未发布任何版本则为空"`
	}
}

func checkVersion(db *bun.DB) func(context.Context, *CheckVersionInput) (*CheckVersionOutput, error) {
	return func(ctx context.Context, in *CheckVersionInput) (*CheckVersionOutput, error) {
		var latest models.AppVersion
		err := db.NewSelect().Model(&latest).
			Order("version_code DESC").
			Limit(1).
			Scan(ctx)
		out := &CheckVersionOutput{}
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return out, nil
			}
			return nil, err
		}
		out.Body.Latest = &latest
		out.Body.HasUpdate = in.Body.VersionCode < latest.VersionCode
		out.Body.ForceUpdate = in.Body.VersionCode < latest.MinSupportedCode
		return out, nil
	}
}

// ---- List

type ListVersionsInput struct {
	Page     int `query:"page" minimum:"1" default:"1" doc:"页码,从 1 开始"`
	PageSize int `query:"page_size" minimum:"1" maximum:"100" default:"20" doc:"每页数量,最大 100"`
}

type ListVersionsOutput struct {
	Body struct {
		Items    []models.AppVersion `json:"items"`
		Total    int                 `json:"total"`
		Page     int                 `json:"page"`
		PageSize int                 `json:"page_size"`
	}
}

func listVersions(db *bun.DB) func(context.Context, *ListVersionsInput) (*ListVersionsOutput, error) {
	return func(ctx context.Context, in *ListVersionsInput) (*ListVersionsOutput, error) {
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

		var items []models.AppVersion
		q := db.NewSelect().Model(&items)
		total, err := q.Count(ctx)
		if err != nil {
			return nil, err
		}
		if err := q.Order("version_code DESC").Limit(pageSize).Offset((page - 1) * pageSize).Scan(ctx); err != nil {
			return nil, err
		}

		out := &ListVersionsOutput{}
		out.Body.Items = items
		out.Body.Total = total
		out.Body.Page = page
		out.Body.PageSize = pageSize
		return out, nil
	}
}

// ---- Get / Create / Update / Delete

type VersionIDInput struct {
	ID int64 `path:"id" doc:"版本记录 ID"`
}

type VersionOutput struct {
	Body models.AppVersion
}

func getVersion(db *bun.DB) func(context.Context, *VersionIDInput) (*VersionOutput, error) {
	return func(ctx context.Context, in *VersionIDInput) (*VersionOutput, error) {
		var v models.AppVersion
		err := db.NewSelect().Model(&v).Where("id = ?", in.ID).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("version not found")
		}
		if err != nil {
			return nil, err
		}
		return &VersionOutput{Body: v}, nil
	}
}

type CreateVersionInput struct {
	Body struct {
		VersionCode      int64  `json:"version_code" required:"true" minimum:"1" example:"12" doc:"版本号,需大于历史最大值"`
		VersionName      string `json:"version_name" required:"true" minLength:"1" maxLength:"32" example:"1.2.0" doc:"版本名"`
		DownloadURL      string `json:"download_url" required:"true" minLength:"1" example:"https://cdn.example.com/app/1.2.0.apk"`
		ReleaseNotes     string `json:"release_notes,omitempty" maxLength:"2000" example:"修复若干问题"`
		MinSupportedCode int64  `json:"min_supported_code,omitempty" minimum:"0" example:"10" doc:"低于该版本号需强制更新,默认 0"`
	}
}

func createVersion(db *bun.DB) func(context.Context, *CreateVersionInput) (*VersionOutput, error) {
	return func(ctx context.Context, in *CreateVersionInput) (*VersionOutput, error) {
		var maxCode int64
		if err := db.NewSelect().
			Model((*models.AppVersion)(nil)).
			ColumnExpr("COALESCE(MAX(version_code), 0)").
			Scan(ctx, &maxCode); err != nil {
			return nil, err
		}
		if in.Body.VersionCode <= maxCode {
			return nil, huma.Error400BadRequest("version_code must be greater than current max")
		}

		now := time.Now()
		v := models.AppVersion{
			VersionCode:      in.Body.VersionCode,
			VersionName:      in.Body.VersionName,
			DownloadURL:      in.Body.DownloadURL,
			ReleaseNotes:     in.Body.ReleaseNotes,
			MinSupportedCode: in.Body.MinSupportedCode,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if _, err := db.NewInsert().Model(&v).Exec(ctx); err != nil {
			return nil, err
		}
		return &VersionOutput{Body: v}, nil
	}
}

type UpdateVersionInput struct {
	Body struct {
		ID               int64   `json:"id" required:"true" doc:"版本记录 ID"`
		VersionName      *string `json:"version_name,omitempty" doc:"版本名"`
		DownloadURL      *string `json:"download_url,omitempty" doc:"下载地址"`
		ReleaseNotes     *string `json:"release_notes,omitempty" doc:"更新说明"`
		MinSupportedCode *int64  `json:"min_supported_code,omitempty" minimum:"0" doc:"最低支持版本号"`
	}
}

func updateVersion(db *bun.DB) func(context.Context, *UpdateVersionInput) (*VersionOutput, error) {
	return func(ctx context.Context, in *UpdateVersionInput) (*VersionOutput, error) {
		var v models.AppVersion
		err := db.NewSelect().Model(&v).Where("id = ?", in.Body.ID).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("version not found")
		}
		if err != nil {
			return nil, err
		}
		if in.Body.VersionName != nil {
			v.VersionName = *in.Body.VersionName
		}
		if in.Body.DownloadURL != nil {
			v.DownloadURL = *in.Body.DownloadURL
		}
		if in.Body.ReleaseNotes != nil {
			v.ReleaseNotes = *in.Body.ReleaseNotes
		}
		if in.Body.MinSupportedCode != nil {
			v.MinSupportedCode = *in.Body.MinSupportedCode
		}
		v.UpdatedAt = time.Now()
		if _, err := db.NewUpdate().Model(&v).WherePK().Exec(ctx); err != nil {
			return nil, err
		}
		return &VersionOutput{Body: v}, nil
	}
}

type DeleteVersionInput struct {
	Body struct {
		ID int64 `json:"id" required:"true" doc:"版本记录 ID"`
	}
}

type DeleteVersionOutput struct {
	Body struct {
		Deleted int64 `json:"deleted"`
	}
}

func deleteVersion(db *bun.DB) func(context.Context, *DeleteVersionInput) (*DeleteVersionOutput, error) {
	return func(ctx context.Context, in *DeleteVersionInput) (*DeleteVersionOutput, error) {
		if _, err := db.NewDelete().Model((*models.AppVersion)(nil)).Where("id = ?", in.Body.ID).Exec(ctx); err != nil {
			return nil, err
		}
		out := &DeleteVersionOutput{}
		out.Body.Deleted = in.Body.ID
		return out, nil
	}
}
