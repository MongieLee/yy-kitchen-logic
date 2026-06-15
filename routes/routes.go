package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"yy-kitchen-logic/config"
	"yy-kitchen-logic/handlers"
	"yy-kitchen-logic/middleware"
)

func Register(r *gin.Engine, conn *gorm.DB, cfg config.Config) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	userH := handlers.NewUserHandler(conn)
	authH := handlers.NewAuthHandler(conn, cfg.JWT)

	api := r.Group("/api/v1")
	{
		api.POST("/auth/login", authH.Login)
		api.POST("/users", userH.Create)
	}

	protected := api.Group("")
	protected.Use(middleware.NewJWTMiddleware(cfg.JWT.Secret))
	{
		protected.GET("/me", userH.Me)
		protected.GET("/users", userH.List)
		protected.GET("/users/:id", userH.Get)
		protected.PUT("/users/:id", userH.Update)
		protected.DELETE("/users/:id", userH.Delete)
	}
}
