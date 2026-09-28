package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"

	"yy-kitchen-logic/config"
	"yy-kitchen-logic/middleware"
	"yy-kitchen-logic/models"
)

// RegisterAppAuthOps registers the APP-side login endpoint.
// APP uses users table + account (phone) + password.
func RegisterAppAuthOps(api huma.API, db *bun.DB, cfg config.JWTConfig) {
	huma.Register(api, huma.Operation{
		OperationID: "app-login",
		Method:      http.MethodPost,
		Path:        "/api/app/v1/auth/login",
		Summary:     "APP 登录",
		Description: "APP 端使用账号(手机号)+ 密码换取 JWT。返回 access token(短期) 与 refresh token(长期)。access token 通过 Authorization: Bearer <token> 头携带。",
		Tags:        []string{"APP-认证"},
	}, appLogin(db, cfg))

	huma.Register(api, huma.Operation{
		OperationID: "app-refresh",
		Method:      http.MethodPost,
		Path:        "/api/app/v1/auth/refresh",
		Summary:     "刷新 access token",
		Description: "使用 refresh token 换取新的 access token + 新的 refresh token(旋转)。refresh token 失效或用户已注销时返回 401。",
		Tags:        []string{"APP-认证"},
	}, appRefresh(db, cfg))
}

// RegisterAdminAuthOps registers the admin-console login endpoint.
// Admin uses admins table + username + password.
func RegisterAdminAuthOps(api huma.API, db *bun.DB, cfg config.JWTConfig) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-login",
		Method:      http.MethodPost,
		Path:        "/api/admin/v1/auth/login",
		Summary:     "后台登录",
		Description: "管理后台使用 username + 密码换取 JWT (作用域 admin)。返回的 token 需通过 Authorization: Bearer <token> 头携带。",
		Tags:        []string{"后台-认证"},
	}, adminLogin(db, cfg))
}

// ---- APP login

type AppLoginInput struct {
	Body struct {
		Account  string `json:"account" required:"true" pattern:"^1[3-9][0-9]{9}$" example:"13800138000" doc:"账号(手机号)"`
		Password string `json:"password" required:"true" example:"strong-password" doc:"密码"`
	}
}

type AppLoginOutput struct {
	Body struct {
		Token        string      `json:"token" doc:"access token,后续请求需在 Authorization 头携带"`
		RefreshToken string      `json:"refresh_token" doc:"refresh token,用于换取新的 access token"`
		User         models.User `json:"user" doc:"当前登录的用户信息"`
	}
}

func appLogin(db *bun.DB, cfg config.JWTConfig) func(context.Context, *AppLoginInput) (*AppLoginOutput, error) {
	return func(ctx context.Context, in *AppLoginInput) (*AppLoginOutput, error) {
		var user models.User
		if err := db.NewSelect().Model(&user).Where("account = ?", in.Body.Account).Scan(ctx); err != nil {
			return nil, huma.Error401Unauthorized("invalid credentials")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(in.Body.Password)); err != nil {
			return nil, huma.Error401Unauthorized("invalid credentials")
		}
		access, refresh, err := signTokenPair(cfg, uint(user.ID), middleware.ScopeApp)
		if err != nil {
			return nil, err
		}
		out := &AppLoginOutput{}
		out.Body.Token = access
		out.Body.RefreshToken = refresh
		out.Body.User = user
		return out, nil
	}
}

// ---- APP refresh

type AppRefreshInput struct {
	Body struct {
		RefreshToken string `json:"refresh_token" required:"true" doc:"登录时返回的 refresh token"`
	}
}

type AppRefreshOutput struct {
	Body struct {
		Token        string `json:"token" doc:"新的 access token"`
		RefreshToken string `json:"refresh_token" doc:"新的 refresh token(旋转)"`
	}
}

func appRefresh(_ *bun.DB, cfg config.JWTConfig) func(context.Context, *AppRefreshInput) (*AppRefreshOutput, error) {
	return func(ctx context.Context, in *AppRefreshInput) (*AppRefreshOutput, error) {
		uid, scope, err := middleware.ParseRefreshToken(in.Body.RefreshToken, cfg.Secret)
		if err != nil {
			return nil, huma.Error401Unauthorized("invalid or expired refresh token")
		}
		if scope != middleware.ScopeApp {
			return nil, huma.Error401Unauthorized("invalid refresh token scope")
		}
		access, refresh, err := signTokenPair(cfg, uid, middleware.ScopeApp)
		if err != nil {
			return nil, err
		}
		out := &AppRefreshOutput{}
		out.Body.Token = access
		out.Body.RefreshToken = refresh
		return out, nil
	}
}

// ---- Admin login

type AdminLoginInput struct {
	Body struct {
		Username string `json:"username" required:"true" example:"admin" doc:"管理员用户名"`
		Password string `json:"password" required:"true" example:"admin" doc:"密码"`
	}
}

type AdminLoginOutput struct {
	Body struct {
		Token string       `json:"token" doc:"JWT,作用域 admin"`
		Admin models.Admin `json:"admin" doc:"当前登录的管理员信息"`
	}
}

func adminLogin(db *bun.DB, cfg config.JWTConfig) func(context.Context, *AdminLoginInput) (*AdminLoginOutput, error) {
	return func(ctx context.Context, in *AdminLoginInput) (*AdminLoginOutput, error) {
		var admin models.Admin
		if err := db.NewSelect().Model(&admin).Where("username = ?", in.Body.Username).Scan(ctx); err != nil {
			return nil, huma.Error401Unauthorized("invalid credentials")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(in.Body.Password)); err != nil {
			return nil, huma.Error401Unauthorized("invalid credentials")
		}
		signed, err := signToken(cfg, uint(admin.ID), middleware.ScopeAdmin)
		if err != nil {
			return nil, err
		}
		out := &AdminLoginOutput{}
		out.Body.Token = signed
		out.Body.Admin = admin
		return out, nil
	}
}

// ---- helpers

func signToken(cfg config.JWTConfig, uid uint, scope string) (string, error) {
	return signTypedToken(cfg, uid, scope, middleware.TokenTypeAccess, cfg.ExpireHours)
}

func signTypedToken(cfg config.JWTConfig, uid uint, scope, tokenType string, expireHours int) (string, error) {
	exp := time.Now().Add(time.Duration(expireHours) * time.Hour)
	claims := middleware.Claims{
		UserID:    uid,
		Scope:     scope,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

func signTokenPair(cfg config.JWTConfig, uid uint, scope string) (access, refresh string, err error) {
	access, err = signTypedToken(cfg, uid, scope, middleware.TokenTypeAccess, cfg.ExpireHours)
	if err != nil {
		return "", "", err
	}
	refreshHours := cfg.RefreshExpireHours
	if refreshHours <= 0 {
		refreshHours = cfg.ExpireHours * 24
	}
	refresh, err = signTypedToken(cfg, uid, scope, middleware.TokenTypeRefresh, refreshHours)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}
