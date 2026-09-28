package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Order is a single "check" placed by a user, containing one or more
// dishes as line items.
// swagger:model
type Order struct {
	bun.BaseModel `bun:"table:orders"`

	ID        int64      `bun:",pk,autoincrement" json:"id"`
	UserID    int64      `bun:",notnull" json:"user_id"`
	Note      string     `bun:"" json:"note"`
	CreatedAt time.Time  `bun:",nullzero" json:"created_at"`
	UpdatedAt time.Time  `bun:",nullzero" json:"updated_at"`
	DeletedAt *time.Time `bun:",soft_delete,nullzero" json:"-"`

	Items []OrderItem `bun:"rel:has-many,join:id=order_id" json:"items"`
}

// OrderItem is a single dish line inside an Order.
// swagger:model
type OrderItem struct {
	bun.BaseModel `bun:"table:order_items"`

	ID       int64 `bun:",pk,autoincrement" json:"id"`
	OrderID  int64 `bun:",notnull" json:"order_id"`
	DishID   int64 `bun:",notnull" json:"dish_id"`
	Quantity int   `bun:",notnull" json:"quantity"`

	Dish *Dish `bun:"rel:belongs-to,join:dish_id=id" json:"dish,omitempty"`
}
