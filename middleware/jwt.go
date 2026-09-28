package middleware

import (
	"context"
	"errors"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	ContextUserIDKey = "user_id"
	ContextScopeKey  = "scope"

	ScopeAdmin = "admin"
	ScopeApp   = "app"

	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type Claims struct {
	UserID    uint   `json:"user_id"`
	Scope     string `json:"scope,omitempty"`
	TokenType string `json:"token_type,omitempty"` // "access" | "refresh"
	jwt.RegisteredClaims
}

// NewJWTMiddleware returns a gin handler that validates a Bearer token
// signed with the given HS256 secret. On success it stores the user id
// and scope in the context.
func NewJWTMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, scope, err := parseBearer(c.GetHeader("Authorization"), secret)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": err.Error()})
			return
		}
		c.Set(ContextUserIDKey, uid)
		c.Set(ContextScopeKey, scope)
		c.Next()
	}
}

// NewHumaJWTMiddleware validates the Bearer token on any operation whose
// Security list references "bearer". It also enforces scope based on the
// operation path prefix: /api/admin/* requires scope=admin, /api/app/*
// requires scope=app.
func NewHumaJWTMiddleware(api huma.API, secret string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		required := false
		for _, s := range ctx.Operation().Security {
			if _, ok := s["bearer"]; ok {
				required = true
				break
			}
		}
		if !required {
			next(ctx)
			return
		}

		uid, scope, err := parseBearer(ctx.Header("Authorization"), secret)
		if err != nil {
			_ = huma.WriteErr(api, ctx, 401, err.Error())
			return
		}

		path := ctx.Operation().Path
		switch {
		case strings.HasPrefix(path, "/api/admin/") && scope != ScopeAdmin:
			_ = huma.WriteErr(api, ctx, 403, "admin scope required")
			return
		case strings.HasPrefix(path, "/api/app/") && scope != ScopeApp:
			_ = huma.WriteErr(api, ctx, 403, "app scope required")
			return
		}

		ctx = huma.WithValue(ctx, ContextUserIDKey, uid)
		ctx = huma.WithValue(ctx, ContextScopeKey, scope)
		newCtx := context.WithValue(ctx.Context(), ContextUserIDKey, uid)
		newCtx = context.WithValue(newCtx, ContextScopeKey, scope)
		ctx = huma.WithContext(ctx, newCtx)
		next(ctx)
	}
}

func parseBearer(header, secret string) (uint, string, error) {
	if header == "" {
		return 0, "", errors.New("missing authorization header")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return 0, "", errors.New("invalid authorization header")
	}
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(parts[1], claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || claims.UserID == 0 {
		return 0, "", errors.New("invalid or expired token")
	}
	// Reject refresh tokens on protected endpoints. Blank type is treated as
	// access for backwards-compat with any tokens issued before this change.
	if claims.TokenType != "" && claims.TokenType != TokenTypeAccess {
		return 0, "", errors.New("invalid or expired token")
	}
	return claims.UserID, claims.Scope, nil
}

// ParseRefreshToken validates a refresh token and returns its (uid, scope).
// It only accepts tokens whose token_type claim is "refresh"; access tokens
// are rejected. Callers should further check the scope matches the caller's
// endpoint prefix (app / admin).
func ParseRefreshToken(rawToken, secret string) (uint, string, error) {
	if rawToken == "" {
		return 0, "", errors.New("missing refresh token")
	}
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(rawToken, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil || claims.UserID == 0 {
		return 0, "", errors.New("invalid or expired refresh token")
	}
	if claims.TokenType != TokenTypeRefresh {
		return 0, "", errors.New("invalid or expired refresh token")
	}
	return claims.UserID, claims.Scope, nil
}
