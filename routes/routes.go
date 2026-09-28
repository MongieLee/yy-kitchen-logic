package routes

import (
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"

	"yy-kitchen-logic/config"
	"yy-kitchen-logic/handlers"
	"yy-kitchen-logic/middleware"
)

// Register wires the health check plus all huma-managed API operations.
// The huma API is mounted on the provided gin engine via humagin, so gin
// middleware and other gin routes continue to work alongside it.
func Register(r *gin.Engine, conn *bun.DB, cfg config.Config) {
	// CORS: 允许任意来源,便于后台/APP/本地开发调用。
	r.Use(cors.New(cors.Config{
		AllowOriginFunc:  func(origin string) bool { return true },
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// APP 侧加密中间件:/api/app/v1/* 需带签名 + AES-GCM 密文 body。
	// 通过 encryption.enabled 开关一键关闭,方便本地/Scalar 直接调试。
	r.Use(middleware.NewEncryptionMiddleware(cfg.Encryption))

	// Base URL middleware: stores request's base URL in context for constructing full URLs.
	r.Use(middleware.BaseURLMiddleware())

	// 文件上传 + 静态访问 (/uploads/*)
	handlers.RegisterUploadRoutes(r)

	humaCfg := huma.DefaultConfig("YY Kitchen Logic API", "1.0.0")
	humaCfg.Info.Title = "YY 菜谱后台 API"
	humaCfg.Info.Description = "yy-kitchen-logic 内部服务接口文档。"
	humaCfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearer": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "在 Authorization 头传入 `Bearer <token>`,token 通过 /api/v1/auth/login 获取。",
		},
	}

	api := humagin.New(r, humaCfg)
	api.UseMiddleware(middleware.NewHumaJWTMiddleware(api, cfg.JWT.Secret))

	// 管理后台接口 (/api/admin/v1/*)
	handlers.RegisterAdminAuthOps(api, conn, cfg.JWT)
	handlers.RegisterAdminUserOps(api, conn)
	handlers.RegisterAdminDishOps(api, conn)
	handlers.RegisterAdminRecommendationOps(api, conn)
	handlers.RegisterAdminVersionOps(api, conn)
	handlers.RegisterAdminFeedbackOps(api, conn)

	// APP 接口 (/api/app/v1/*)
	handlers.RegisterAppAuthOps(api, conn, cfg.JWT)
	handlers.RegisterAppUserOps(api, conn)
	handlers.RegisterAppDishOps(api, conn)
	handlers.RegisterAppRecommendationOps(api, conn)
	handlers.RegisterAppOrderOps(api, conn)
	handlers.RegisterAppFeedbackOps(api, conn)
	handlers.RegisterAppVersionOps(api, conn)
}
