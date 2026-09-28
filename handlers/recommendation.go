package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/uptrace/bun"

	"yy-kitchen-logic/middleware"
	"yy-kitchen-logic/models"
)

// RegisterAdminRecommendationOps registers recommendation CRUD on the admin API.
func RegisterAdminRecommendationOps(api huma.API, db *bun.DB) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-list-recommendations",
		Method:      http.MethodGet,
		Path:        "/api/admin/v1/recommendations",
		Summary:     "今日推荐列表",
		Description: "返回所有推荐记录，按 sort_order 排序。",
		Tags:        []string{"后台-今日推荐"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, listRecommendations(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-get-recommendation",
		Method:      http.MethodGet,
		Path:        "/api/admin/v1/recommendations/{id}",
		Summary:     "今日推荐详情",
		Description: "根据 ID 获取推荐详情。",
		Tags:        []string{"后台-今日推荐"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, getRecommendation(db))

	huma.Register(api, huma.Operation{
		OperationID:   "admin-create-recommendation",
		Method:        http.MethodPost,
		Path:          "/api/admin/v1/recommendations/create",
		Summary:       "新增今日推荐",
		Description:   "创建一条今日推荐记录。",
		Tags:          []string{"后台-今日推荐"},
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: http.StatusCreated,
	}, createRecommendation(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-update-recommendation",
		Method:      http.MethodPost,
		Path:        "/api/admin/v1/recommendations/update",
		Summary:     "更新今日推荐",
		Description: "字段均为可选，仅对传入字段做更新。id 必填。",
		Tags:        []string{"后台-今日推荐"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, updateRecommendation(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-delete-recommendation",
		Method:      http.MethodPost,
		Path:        "/api/admin/v1/recommendations/delete",
		Summary:     "删除今日推荐",
		Description: "软删除指定推荐记录。id 在 body 中传入。",
		Tags:        []string{"后台-今日推荐"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, deleteRecommendation(db))
}

// RegisterAppRecommendationOps registers recommendation query for the APP.
func RegisterAppRecommendationOps(api huma.API, db *bun.DB) {
	huma.Register(api, huma.Operation{
		OperationID: "app-get-active-recommendations",
		Method:      http.MethodGet,
		Path:        "/api/app/v1/recommendations",
		Summary:     "今日推荐列表",
		Description: "返回当前激活的推荐列表，按 sort_order 排序。",
		Tags:        []string{"APP-今日推荐"},
	}, listActiveRecommendations(db))
}

// ---- Shared types

type RecommendationOutput struct {
	Body models.Recommendation
}

// ---- Admin: List all

type ListRecommendationsOutput struct {
	Body struct {
		Items []models.Recommendation `json:"items"`
		Total int                     `json:"total"`
	}
}

func listRecommendations(db *bun.DB) func(context.Context, *struct{}) (*ListRecommendationsOutput, error) {
	return func(ctx context.Context, _ *struct{}) (*ListRecommendationsOutput, error) {
		var items []models.Recommendation
		total, err := db.NewSelect().Model(&items).Count(ctx)
		if err != nil {
			return nil, err
		}
		if err := db.NewSelect().Model(&items).Order("sort_order ASC, id DESC").Scan(ctx); err != nil {
			return nil, err
		}
		out := &ListRecommendationsOutput{}
		out.Body.Items = items
		out.Body.Total = total
		return out, nil
	}
}

// ---- Admin: Get by ID

type RecommendationIDInput struct {
	ID int64 `path:"id" doc:"推荐 ID"`
}

func getRecommendation(db *bun.DB) func(context.Context, *RecommendationIDInput) (*RecommendationOutput, error) {
	return func(ctx context.Context, in *RecommendationIDInput) (*RecommendationOutput, error) {
		var rec models.Recommendation
		err := db.NewSelect().Model(&rec).Where("id = ?", in.ID).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("recommendation not found")
		}
		if err != nil {
			return nil, err
		}
		return &RecommendationOutput{Body: rec}, nil
	}
}

// ---- Admin: Create

type CreateRecommendationInput struct {
	Body struct {
		Name      string `json:"name" doc:"推荐菜品名称"`
		ChefNote  string `json:"chef_note" doc:"厨师寄语"`
		ImageURL  string `json:"image_url" doc:"图片 URL"`
		Active    *bool  `json:"active,omitempty" doc:"是否激活，默认 true"`
		SortOrder *int   `json:"sort_order,omitempty" doc:"排序权重，越小越靠前"`
	}
}

func createRecommendation(db *bun.DB) func(context.Context, *CreateRecommendationInput) (*RecommendationOutput, error) {
	return func(ctx context.Context, in *CreateRecommendationInput) (*RecommendationOutput, error) {
		now := time.Now()
		rec := models.Recommendation{
			Name:      in.Body.Name,
			ChefNote:  in.Body.ChefNote,
			ImageURL:  in.Body.ImageURL,
			Active:    true,
			SortOrder: 0,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if in.Body.Active != nil {
			rec.Active = *in.Body.Active
		}
		if in.Body.SortOrder != nil {
			rec.SortOrder = *in.Body.SortOrder
		}
		if _, err := db.NewInsert().Model(&rec).Exec(ctx); err != nil {
			return nil, err
		}
		return &RecommendationOutput{Body: rec}, nil
	}
}

// ---- Admin: Update

type UpdateRecommendationInput struct {
	Body struct {
		ID        int64   `json:"id" required:"true" doc:"推荐 ID"`
		Name      *string `json:"name,omitempty" doc:"推荐菜品名称"`
		ChefNote  *string `json:"chef_note,omitempty" doc:"厨师寄语"`
		ImageURL  *string `json:"image_url,omitempty" doc:"图片 URL"`
		Active    *bool   `json:"active,omitempty" doc:"是否激活"`
		SortOrder *int    `json:"sort_order,omitempty" doc:"排序权重"`
	}
}

func updateRecommendation(db *bun.DB) func(context.Context, *UpdateRecommendationInput) (*RecommendationOutput, error) {
	return func(ctx context.Context, in *UpdateRecommendationInput) (*RecommendationOutput, error) {
		var rec models.Recommendation
		err := db.NewSelect().Model(&rec).Where("id = ?", in.Body.ID).Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("recommendation not found")
		}
		if err != nil {
			return nil, err
		}
		if in.Body.Name != nil {
			rec.Name = *in.Body.Name
		}
		if in.Body.ChefNote != nil {
			rec.ChefNote = *in.Body.ChefNote
		}
		if in.Body.ImageURL != nil {
			rec.ImageURL = *in.Body.ImageURL
		}
		if in.Body.Active != nil {
			rec.Active = *in.Body.Active
		}
		if in.Body.SortOrder != nil {
			rec.SortOrder = *in.Body.SortOrder
		}
		rec.UpdatedAt = time.Now()
		if _, err := db.NewUpdate().Model(&rec).WherePK().Exec(ctx); err != nil {
			return nil, err
		}
		return &RecommendationOutput{Body: rec}, nil
	}
}

// ---- Admin: Delete

type DeleteRecommendationInput struct {
	Body struct {
		ID int64 `json:"id" required:"true" doc:"推荐 ID"`
	}
}

type DeleteRecommendationOutput struct {
	Body struct {
		Deleted int64 `json:"deleted"`
	}
}

func deleteRecommendation(db *bun.DB) func(context.Context, *DeleteRecommendationInput) (*DeleteRecommendationOutput, error) {
	return func(ctx context.Context, in *DeleteRecommendationInput) (*DeleteRecommendationOutput, error) {
		if _, err := db.NewDelete().Model((*models.Recommendation)(nil)).Where("id = ?", in.Body.ID).Exec(ctx); err != nil {
			return nil, err
		}
		out := &DeleteRecommendationOutput{}
		out.Body.Deleted = in.Body.ID
		return out, nil
	}
}

// ---- App: List active recommendations

type AppRecommendationsOutput struct {
	Body struct {
		Items []models.Recommendation `json:"items"`
	}
}

func listActiveRecommendations(db *bun.DB) func(context.Context, *struct{}) (*AppRecommendationsOutput, error) {
	return func(ctx context.Context, _ *struct{}) (*AppRecommendationsOutput, error) {
		var items []models.Recommendation
		if err := db.NewSelect().Model(&items).
			Where("active = ?", true).
			Order("sort_order ASC, id DESC").
			Scan(ctx); err != nil {
			return nil, err
		}

		// Get base URL from context to construct full image URLs
		baseURL, _ := ctx.Value(middleware.ContextBaseURLKey).(string)

		// Prepend base URL to relative image paths
		for i := range items {
			if baseURL != "" && items[i].ImageURL != "" && strings.HasPrefix(items[i].ImageURL, "/") {
				items[i].ImageURL = baseURL + items[i].ImageURL
			}
		}

		out := &AppRecommendationsOutput{}
		out.Body.Items = items
		return out, nil
	}
}
