package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/auth"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type contextKey string

const ContextKeyClaims contextKey = "claims"

type ClaimsContext struct {
	UserID uuid.UUID
	Email  string
	Role   model.UserRole
}

func Auth(jwtSecret string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}
			claims, err := auth.ParseToken(parts[1], jwtSecret)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := contextWithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth attaches JWT claims when a valid Bearer token is present; otherwise the request continues anonymously.
func OptionalAuth(jwtSecret string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header != "" {
				parts := strings.SplitN(header, " ", 2)
				if len(parts) == 2 && parts[0] == "Bearer" {
					if claims, err := auth.ParseToken(parts[1], jwtSecret); err == nil {
						r = r.WithContext(contextWithClaims(r.Context(), claims))
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func contextWithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, ContextKeyClaims, &ClaimsContext{
		UserID: claims.UserID,
		Email:  claims.Email,
		Role:   claims.Role,
	})
}

func GetClaims(ctx context.Context) *ClaimsContext {
	c, _ := ctx.Value(ContextKeyClaims).(*ClaimsContext)
	return c
}
