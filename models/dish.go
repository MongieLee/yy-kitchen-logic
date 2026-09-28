package models

import (
	"time"

	"github.com/uptrace/bun"
)

// DishCategory enumerates the supported menu categories.
type DishCategory int

const (
	DishCategoryStaple  DishCategory = 1 // 主食
	DishCategoryMain    DishCategory = 2 // 主菜
	DishCategoryDrink   DishCategory = 3 // 饮料
	DishCategoryDessert DishCategory = 4 // 甜点
)

// IsValid reports whether c is a known DishCategory.
func (c DishCategory) IsValid() bool {
	switch c {
	case DishCategoryStaple, DishCategoryMain, DishCategoryDrink, DishCategoryDessert:
		return true
	}
	return false
}

// Dish represents a menu item that can be ordered.
// swagger:model
type Dish struct {
	bun.BaseModel `bun:"table:dishes"`

	ID          int64        `bun:",pk,autoincrement" json:"id"`
	CreatedAt   time.Time    `bun:",nullzero" json:"created_at"`
	UpdatedAt   time.Time    `bun:",nullzero" json:"updated_at"`
	DeletedAt   *time.Time   `bun:",soft_delete,nullzero" json:"-"`
	Name        string       `bun:",notnull" json:"name" example:"宫保鸡丁"`
	Description string       `bun:",notnull" json:"description" example:"经典川菜,微辣"`
	Icon        string       `bun:",notnull" json:"icon" example:"https://cdn.example.com/dishes/kungpao.png"`
	Category    DishCategory `bun:",notnull" json:"category" example:"2"` // 1主食 2主菜 3饮料 4甜点
}
