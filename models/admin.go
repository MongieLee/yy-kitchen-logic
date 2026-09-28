package models

import (
	"time"

	"github.com/uptrace/bun"
)

// Admin represents an operator that can log in to the management console.
// swagger:model
type Admin struct {
	bun.BaseModel `bun:"table:admins"`

	ID        int64      `bun:",pk,autoincrement" json:"id"`
	Username  string     `bun:",notnull,unique" json:"username" example:"admin"`
	Password  string     `bun:",notnull" json:"-"`
	Name      string     `bun:"" json:"name" example:"超级管理员"`
	CreatedAt time.Time  `bun:",nullzero" json:"created_at"`
	UpdatedAt time.Time  `bun:",nullzero" json:"updated_at"`
	DeletedAt *time.Time `bun:",soft_delete,nullzero" json:"-"`
}
