// Package main bootstraps the yy-kitchen-logic HTTP server.
package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"yy-kitchen-logic/config"
	"yy-kitchen-logic/db"
	"yy-kitchen-logic/models"
	"yy-kitchen-logic/routes"
)

func main() {
	cfg := config.Load()
	gin.SetMode(cfg.App.Mode)

	conn := db.Init(cfg.Database,
		&models.User{},
		&models.Admin{},
		&models.Dish{},
		&models.Recommendation{},
		&models.Order{},
		&models.OrderItem{},
		&models.Feedback{},
		&models.AppVersion{},
	)
	db.SeedDefaults(conn)

	r := gin.Default()

	routes.Register(r, conn, *cfg)

	addr := ":" + cfg.App.Port
	log.Printf("server starting on %s (docs at %s/docs, openapi at %s/openapi.json)", addr, addr, addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
