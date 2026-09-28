package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/uptrace/bun"

	"yy-kitchen-logic/middleware"
	"yy-kitchen-logic/models"
)

// RegisterAppOrderOps registers the ordering + history endpoints on the APP API.
func RegisterAppOrderOps(api huma.API, db *bun.DB) {
	huma.Register(api, huma.Operation{
		OperationID:   "app-create-order",
		Method:        http.MethodPost,
		Path:          "/api/app/v1/orders/create",
		Summary:       "下单(点菜)",
		Description:   "为当前登录用户创建一笔订单,支持一次点多道菜。",
		Tags:          []string{"APP-订单"},
		Security:      []map[string][]string{{"bearer": {}}},
		DefaultStatus: http.StatusCreated,
	}, createOrder(db))

	huma.Register(api, huma.Operation{
		OperationID: "app-list-orders",
		Method:      http.MethodGet,
		Path:        "/api/app/v1/orders",
		Summary:     "点菜历史",
		Description: "返回当前登录用户的历史订单(按下单时间倒序),支持分页 + 关键字/日期过滤。每笔订单会展开对应的菜品信息。",
		Tags:        []string{"APP-订单"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, listOrders(db))
}

// ---- Create

type CreateOrderInput struct {
	Body struct {
		Note  string `json:"note,omitempty" doc:"备注(可选)"`
		Items []struct {
			DishID   int64 `json:"dish_id" required:"true" minimum:"1" doc:"菜品 ID"`
			Quantity int   `json:"quantity" required:"true" minimum:"1" maximum:"999" doc:"数量,至少 1"`
		} `json:"items" required:"true" minItems:"1" doc:"点单明细,至少一条"`
	}
}

type OrderOutput struct {
	Body models.Order
}

func createOrder(db *bun.DB) func(context.Context, *CreateOrderInput) (*OrderOutput, error) {
	return func(ctx context.Context, in *CreateOrderInput) (*OrderOutput, error) {
		uid, ok := ctx.Value(middleware.ContextUserIDKey).(uint)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}

		// Collect requested dish ids and verify they all exist.
		dishIDs := make([]int64, 0, len(in.Body.Items))
		for _, it := range in.Body.Items {
			dishIDs = append(dishIDs, it.DishID)
		}
		count, err := db.NewSelect().Model((*models.Dish)(nil)).
			Where("id IN (?)", bun.In(dishIDs)).
			Count(ctx)
		if err != nil {
			return nil, err
		}
		if count != len(uniqueInt64(dishIDs)) {
			return nil, huma.Error400BadRequest("one or more dish_id do not exist")
		}

		now := time.Now()
		order := models.Order{
			UserID:    int64(uid),
			Note:      in.Body.Note,
			CreatedAt: now,
			UpdatedAt: now,
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback() //nolint:errcheck

		if _, err := tx.NewInsert().Model(&order).Exec(ctx); err != nil {
			return nil, err
		}

		items := make([]models.OrderItem, 0, len(in.Body.Items))
		for _, it := range in.Body.Items {
			items = append(items, models.OrderItem{
				OrderID:  order.ID,
				DishID:   it.DishID,
				Quantity: it.Quantity,
			})
		}
		if _, err := tx.NewInsert().Model(&items).Exec(ctx); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}

		// Re-fetch with relations for a rich response.
		full, err := loadOrder(ctx, db, order.ID, int64(uid))
		if err != nil {
			return nil, err
		}
		return &OrderOutput{Body: *full}, nil
	}
}

// ---- List (history)

type ListOrdersInput struct {
	Keyword   string `query:"keyword" doc:"关键字,匹配订单备注或所点菜品名称(模糊匹配)"`
	StartDate string `query:"start_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" doc:"起始日期 (YYYY-MM-DD),含当日"`
	EndDate   string `query:"end_date" pattern:"^[0-9]{4}-[0-9]{2}-[0-9]{2}$" doc:"结束日期 (YYYY-MM-DD),含当日"`
	Page      int    `query:"page" minimum:"1" default:"1" doc:"页码,从 1 开始"`
	PageSize  int    `query:"page_size" minimum:"1" maximum:"100" default:"20" doc:"每页数量,最大 100"`
}

type ListOrdersOutput struct {
	Body struct {
		Items       []models.Order `json:"items"`
		Total       int            `json:"total" doc:"当前过滤条件下的匹配数"`
		TotalOrders int            `json:"total_orders" doc:"用户全部历史点菜次数(忽略过滤条件)"`
		Page        int            `json:"page"`
		PageSize    int            `json:"page_size"`
	}
}

func listOrders(db *bun.DB) func(context.Context, *ListOrdersInput) (*ListOrdersOutput, error) {
	return func(ctx context.Context, in *ListOrdersInput) (*ListOrdersOutput, error) {
		uid, ok := ctx.Value(middleware.ContextUserIDKey).(uint)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthenticated")
		}

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

		var orders []models.Order
		q := db.NewSelect().Model(&orders).Where("user_id = ?", int64(uid))

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
			// end of day (inclusive): add 24h and use strict less-than.
			q = q.Where("created_at < ?", end.Add(24*time.Hour))
		}
		if kw := strings.TrimSpace(in.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where(
				"note LIKE ? OR EXISTS (SELECT 1 FROM order_items oi JOIN dishes d ON d.id = oi.dish_id WHERE oi.order_id = ?TableAlias.id AND d.name LIKE ?)",
				like, like,
			)
		}

		total, err := q.Count(ctx)
		if err != nil {
			return nil, err
		}
		totalOrders, err := db.NewSelect().Model((*models.Order)(nil)).
			Where("user_id = ?", int64(uid)).Count(ctx)
		if err != nil {
			return nil, err
		}
		if err := q.
			Relation("Items").
			Relation("Items.Dish").
			Order("id DESC").
			Limit(pageSize).
			Offset((page - 1) * pageSize).
			Scan(ctx); err != nil {
			return nil, err
		}

		out := &ListOrdersOutput{}
		out.Body.Items = orders
		out.Body.Total = total
		out.Body.TotalOrders = totalOrders
		out.Body.Page = page
		out.Body.PageSize = pageSize
		return out, nil
	}
}

// ---- helpers

func loadOrder(ctx context.Context, db *bun.DB, id, userID int64) (*models.Order, error) {
	var order models.Order
	err := db.NewSelect().Model(&order).
		Where("id = ?", id).
		Where("user_id = ?", userID).
		Relation("Items").
		Relation("Items.Dish").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func uniqueInt64(in []int64) []int64 {
	seen := make(map[int64]struct{}, len(in))
	out := make([]int64, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
