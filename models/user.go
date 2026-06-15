package models

import "gorm.io/gorm"

// User represents an account that can authenticate against the API.
// swagger:model
type User struct {
	gorm.Model
	Name     string `json:"name" gorm:"size:100;not null" example:"alice"`
	Email    string `json:"email" gorm:"size:100;uniqueIndex;not null" example:"alice@example.com"`
	Password string `json:"-" gorm:"size:255;not null" example:"strong-password"`
}
