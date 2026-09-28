package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ContextKey is a type for context keys to avoid collisions.
type ContextKey string

const (
	// ContextBaseURLKey is the context key for the request's base URL (scheme + host).
	ContextBaseURLKey ContextKey = "base_url"
)

// BaseURLMiddleware stores the request's base URL (scheme + host) in the request context.
// This allows handlers to construct full URLs for resources like uploaded files.
func BaseURLMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		// Check X-Forwarded-Proto header (common in reverse proxy setups)
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		}
		baseURL := scheme + "://" + c.Request.Host
		// Store in gin context for gin handlers
		c.Set(string(ContextBaseURLKey), baseURL)
		// Store in request context for huma handlers
		ctx := context.WithValue(c.Request.Context(), ContextBaseURLKey, baseURL)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// GetBaseURL extracts the base URL from the request context.
func GetBaseURL(r *http.Request) string {
	if v, ok := r.Context().Value(ContextBaseURLKey).(string); ok {
		return v
	}
	return ""
}
