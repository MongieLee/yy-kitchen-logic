package db

import (
	"context"
	"log"
	"time"

	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"

	"yy-kitchen-logic/models"
)

// SeedDefaults ensures a default admin (admin/admin) exists in admins, and a
// default APP user (lemon / 13232251037 / 123456) exists in users. Both are
// no-ops if the record already exists.
func SeedDefaults(b *bun.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := seedAdmin(ctx, b, "admin", "admin", "超级管理员"); err != nil {
		log.Fatalf("seed admin failed: %v", err)
	}
	if err := seedAppUser(ctx, b, "13232251037", "123456", "lemon"); err != nil {
		log.Fatalf("seed app user failed: %v", err)
	}
	seedDefaultRecommendations(ctx, b)
}

func seedAdmin(ctx context.Context, b *bun.DB, username, password, name string) error {
	count, err := b.NewSelect().Model((*models.Admin)(nil)).Where("username = ?", username).Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	now := time.Now()
	admin := models.Admin{
		Username:  username,
		Password:  string(hash),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err = b.NewInsert().Model(&admin).Exec(ctx)
	if err == nil {
		log.Printf("seeded default admin: %s / %s", username, password)
	}
	return err
}

func seedAppUser(ctx context.Context, b *bun.DB, account, password, name string) error {
	count, err := b.NewSelect().Model((*models.User)(nil)).Where("account = ?", account).Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	now := time.Now()
	user := models.User{
		Name:      name,
		Account:   account,
		Password:  string(hash),
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err = b.NewInsert().Model(&user).Exec(ctx)
	if err == nil {
		log.Printf("seeded default app user: %s / %s (name=%s)", account, password, name)
	}
	return err
}

func seedDefaultRecommendations(ctx context.Context, b *bun.DB) {
	count, err := b.NewSelect().Model((*models.Recommendation)(nil)).Count(ctx)
	if err != nil {
		log.Printf("warning: seed recommendations count failed: %v", err)
		return
	}
	if count > 0 {
		return
	}

	now := time.Now()
	recs := []models.Recommendation{
		{
			Name:      "爱心舒芙蕾松饼",
			ChefNote:  "满满都是爱，只为你而做。",
			ImageURL:  "",
			Active:    true,
			SortOrder: 1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			Name:      "彩虹水果沙拉",
			ChefNote:  "新鲜水果，元气满满的一天！",
			ImageURL:  "",
			Active:    true,
			SortOrder: 2,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if _, err := b.NewInsert().Model(&recs).Exec(ctx); err != nil {
		log.Printf("warning: seed recommendations failed: %v", err)
		return
	}
	log.Printf("seeded %d default recommendations", len(recs))
}
