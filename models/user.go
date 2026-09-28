package models

import (
	"time"

	"github.com/uptrace/bun"
)

// User represents an account that can authenticate against the API.
// swagger:model
type User struct {
	bun.BaseModel `bun:"table:users"`

	ID        int64      `bun:",pk,autoincrement" json:"id"`
	CreatedAt time.Time  `bun:",nullzero" json:"created_at"`
	UpdatedAt time.Time  `bun:",nullzero" json:"updated_at"`
	DeletedAt *time.Time `bun:",soft_delete,nullzero" json:"-"`
	Name      string     `bun:",notnull" json:"name" example:"lemon"`
	Avatar    string     `bun:"" json:"avatar" example:"/uploads/xxx.png"`
	Account   string     `bun:",notnull,unique" json:"account" example:"13800138000"`
	Password  string     `bun:",notnull" json:"-" example:"strong-password"`
}
