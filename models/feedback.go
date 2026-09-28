package models

import (
	"time"

	"github.com/uptrace/bun"
)

// FeedbackCategory enumerates supported feedback types.
type FeedbackCategory int

const (
	FeedbackCategoryBug        FeedbackCategory = 1 // bug 反馈
	FeedbackCategorySuggestion FeedbackCategory = 2 // 建议
	FeedbackCategoryOther      FeedbackCategory = 3 // 其他
)

// IsValid reports whether c is a known FeedbackCategory.
func (c FeedbackCategory) IsValid() bool {
	switch c {
	case FeedbackCategoryBug, FeedbackCategorySuggestion, FeedbackCategoryOther:
		return true
	}
	return false
}

// Feedback is a problem/suggestion submitted by a logged-in user.
// swagger:model
type Feedback struct {
	bun.BaseModel `bun:"table:feedbacks"`

	ID        int64            `bun:",pk,autoincrement" json:"id"`
	UserID    int64            `bun:",notnull" json:"user_id"`
	Category  FeedbackCategory `bun:",notnull" json:"category" example:"1"` // 1 bug 2 建议 3 其他
	Content   string           `bun:",notnull" json:"content" example:"下单页偶尔白屏"`
	CreatedAt time.Time        `bun:",nullzero" json:"created_at"`
	UpdatedAt time.Time        `bun:",nullzero" json:"updated_at"`
	DeletedAt *time.Time       `bun:",soft_delete,nullzero" json:"-"`

	User *User `bun:"rel:belongs-to,join:user_id=id" json:"user,omitempty"`
}
