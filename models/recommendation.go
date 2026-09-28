package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Recommendation represents a daily featured dish shown on the home screen.
// swagger:model
type Recommendation struct {
	bun.BaseModel `bun:"table:recommendations"`

	ID        int64      `bun:",pk,autoincrement" json:"id"`
	CreatedAt time.Time  `bun:",nullzero" json:"created_at"`
	UpdatedAt time.Time  `bun:",nullzero" json:"updated_at"`
	DeletedAt *time.Time `bun:"soft_delete,nullzero" json:"-"`

	// DishID references the dish being featured (optional, can be null for custom recommendations).
	DishID *int64 `bun:",nullzero" json:"dish_id"`
	// Name is the display name of the featured dish (e.g. "爱心舒芙蕾松饼").
	Name string `bun:",notnull" json:"name"`
	// ChefNote is the chef's message shown alongside the dish (e.g. "满满都是爱，只为你而做。").
	ChefNote string `bun:",notnull" json:"chef_note"`
	// ImageURL is the image of the featured dish.
	ImageURL string `bun:",notnull" json:"image_url"`
	// Active indicates whether this recommendation is currently being shown.
	Active bool `bun:",notnull,default:true" json:"active"`
	// SortOrder controls display order (lower = higher priority).
	SortOrder int `bun:",notnull,default:0" json:"sort_order"`
}
