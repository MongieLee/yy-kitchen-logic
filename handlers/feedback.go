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

// RegisterAppFeedbackOps registers feedback endpoints on the APP API.
func RegisterAppFeedbackOps(api huma.API, db *bun.DB) {
	huma.Register(api, huma.Operation{
		OperationID:   "app-create-feedback",
		Method:        http.MethodPost,
		Path:          "/api/app/v1/feedback/create",
		Summary:       "提交反馈",
		Description:   "当前登录用户提交一条问题反馈,需指定类型 (1 bug / 2 建议 / 3 其他) 与详情内容。",
		Tags:          []string{"APP-反馈"},
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: http.StatusCreated,
	}, createFeedback(db))
}

// RegisterAdminFeedbackOps registers feedback query endpoints on the admin API.
func RegisterAdminFeedbackOps(api huma.API, db *bun.DB) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-list-feedbacks",
		Method:      http.MethodGet,
		Path:        "/api/admin/v1/feedback",
		Summary:     "反馈列表",
		Description: "按创建时间倒序返回问题反馈,支持类型/关键字/日期/用户过滤 + 分页。展开对应用户信息。",
		Tags:        []string{"后台-反馈"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, listFeedbacks(db))

	huma.Register(api, huma.Operation{
		OperationID: "admin-get-feedback",
		Method:      http.MethodGet,
		Path:        "/api/admin/v1/feedback/{id}",
		Summary:     "反馈详情",
		Description: "根据反馈 ID 获取详情,展开提交用户。",
		Tags:        []string{"后台-反馈"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, getFeedback(db))
}

// ---- App: create

type CreateFeedbackInput struct {
	Body struct {
		Category models.FeedbackCategory `json:"category" required:"true" minimum:"1" maximum:"3" example:"1" doc:"反馈类型: 1 bug 反馈 2 建议 3 其他"`
		Content  string                  `json:"content" required:"true" minLength:"1" maxLength:"2000" example:"下单页偶尔白屏" doc:"反馈详情"`
	}
}

type FeedbackOutput struct {
	Body models.Feedback
}

func createFeedback(db *bun.DB) func(context.Context, *CreateFeedbackInput) (*FeedbackOutput, error) {
	return func(ctx context.Context, in *CreateFeedbackInput) (*FeedbackOutput, error) {
		uid, ok := ctx.Value(middleware.ContextUserIDKey).(uint)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}
		if !in.Body.Category.IsValid() {
			return nil, huma.Error400BadRequest("invalid category")
		}

		now := time.Now()
		fb := models.Feedback{
			UserID:    int64(uid),
			Category:  in.Body.Category,
			Content:   in.Body.Content,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if _, err := db.NewInsert().Model(&fb).Exec(ctx); err != nil {
			return nil, err
		}
		return &FeedbackOutput{Body: fb}, nil
	}
}

// ---- Admin: list

type ListFeedbacksInput struct {
	Category  int    `query:"category" minimum:"1" maximum:"3" doc:"反馈类型:1 bug / 2 建议 / 3 其他。留空不过滤。"`
	UserID    int64  `query:"user_id" minimum:"1" doc:"按提交用户 ID 过滤"`
	Keyword   string `query:"keyword" doc:"关键字,匹配反馈内容(模糊)"`
	StartDate string `query:"start_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" doc:"起始日期 YYYY-MM-DD (含当日)"`
	EndDate   string `query:"end_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" doc:"结束日期 YYYY-MM-DD (含当日)"`
	Page      int    `query:"page" minimum:"1" default:"1" doc:"页码,从 1 开始"`
	PageSize  int    `query:"page_size" minimum:"1" maximum:"100" default:"20" doc:"每页数量,最大 100"`
}

type ListFeedbacksOutput struct {
	Body struct {
		Items    []models.Feedback `json:"items"`
		Total    int               `json:"total"`
		Page     int               `json:"page"`
		PageSize int               `json:"page_size"`
	}
}

func listFeedbacks(db *bun.DB) func(context.Context, *ListFeedbacksInput) (*ListFeedbacksOutput, error) {
	return func(ctx context.Context, in *ListFeedbacksInput) (*ListFeedbacksOutput, error) {
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

		var items []models.Feedback
		q := db.NewSelect().Model(&items)

		if in.Category != 0 {
			if !models.FeedbackCategory(in.Category).IsValid() {
				return nil, huma.Error400BadRequest("invalid category")
			}
			q = q.Where("category = ?", in.Category)
		}
		if in.UserID != 0 {
			q = q.Where("user_id = ?", in.UserID)
		}
		if kw := strings.TrimSpace(in.Keyword); kw != "" {
			q = q.Where("content LIKE ?", "%"+kw+"%")
		}
		if in.StartDate != "" {
			start, err := time.ParseInLocation("2006-01-02", in.StartDate, time.Local)
			if err != nil {
				return nil, huma.Error400BadRequest("invalid start_date")
			}
			q = q.Where("created_at >= ?", start)
		}
		if in.EndDate != "" {
			end, err := time.ParseInLocation("2006-01-02", in.EndDate, time.Local)
			if err != nil {
				return nil, huma.Error400BadRequest("invalid end_date")
			}
			q = q.Where("created_at < ?", end.Add(24*time.Hour))
		}

		total, err := q.Count(ctx)
		if err != nil {
			return nil, err
		}
		if err := q.Relation("User").
			Order("id DESC").
			Limit(pageSize).
			Offset((page - 1) * pageSize).
			Scan(ctx); err != nil {
			return nil, err
		}

		out := &ListFeedbacksOutput{}
		out.Body.Items = items
		out.Body.Total = total
		out.Body.Page = page
		out.Body.PageSize = pageSize
		return out, nil
	}
}

// ---- Admin: get

type FeedbackIDInput struct {
	ID int64 `path:"id" doc:"反馈 ID"`
}

func getFeedback(db *bun.DB) func(context.Context, *FeedbackIDInput) (*FeedbackOutput, error) {
	return func(ctx context.Context, in *FeedbackIDInput) (*FeedbackOutput, error) {
		var fb models.Feedback
		err := db.NewSelect().Model(&fb).Where("feedback.id = ?", in.ID).Relation("User").Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, huma.Error404NotFound("feedback not found")
		}
		if err != nil {
			return nil, err
		}
		return &FeedbackOutput{Body: fb}, nil
	}
}
