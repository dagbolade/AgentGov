package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// User represents an authenticated user
type User struct {
	ID       string   `json:"id"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Roles    []string `json:"roles"`
	IssuedAt int64    `json:"iat"`
}

// Claims extends JWT standard claims
type Claims struct {
	User User `json:"user"`
	jwt.RegisteredClaims
}

// Config holds auth configuration
type Config struct {
	JWTSecret       string
	TokenExpiration time.Duration
	RequireAuth     bool
	AllowedRoles    []string
}

// Manager handles authentication
type Manager struct {
	config Config
	secret []byte
}

// NewManager creates auth manager
func NewManager(config Config) *Manager {
	secret := config.JWTSecret
	if secret == "" {
		secret = os.Getenv("JWT_SECRET")
	}
	if secret == "" {
		// Generate random secret (dev only)
		b := make([]byte, 32)
		rand.Read(b)
		secret = base64.StdEncoding.EncodeToString(b)
		log.Warn().Msg("Using generated JWT secret. Set JWT_SECRET env var for production.")
	}

	return &Manager{
		config: config,
		secret: []byte(secret),
	}
}

// Middleware returns Echo middleware for authentication
func (m *Manager) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip auth if not required
			if !m.config.RequireAuth {
				return next(c)
			}

			// Skip auth for public endpoints
			path := c.Path()
			if path == "/health" || path == "/login" {
				return next(c)
			}

			// Extract token from Authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(401, map[string]string{
					"error": "Missing authorization header",
				})
			}

			// Parse Bearer token
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return c.JSON(401, map[string]string{
					"error": "Invalid authorization header format",
				})
			}

			// Validate token
			user, err := m.ValidateToken(parts[1])
			if err != nil {
				return c.JSON(401, map[string]string{
					"error": fmt.Sprintf("Invalid token: %v", err),
				})
			}

			// Check role requirements
			if len(m.config.AllowedRoles) > 0 {
				if !m.hasRequiredRole(user) {
					return c.JSON(403, map[string]string{
						"error": "Insufficient permissions",
					})
				}
			}

			// Add user to context
			c.Set("user", user)
			return next(c)
		}
	}
}

// RequireRole returns middleware that checks for specific role
func (m *Manager) RequireRole(role string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := GetUserFromContext(c)
			if user == nil {
				return c.JSON(401, map[string]string{
					"error": "Authentication required",
				})
			}

			// Check if user has required role
			hasRole := false
			for _, userRole := range user.Roles {
				if userRole == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				return c.JSON(403, map[string]string{
					"error": fmt.Sprintf("Role '%s' required", role),
				})
			}

			return next(c)
		}
	}
}

// GenerateToken creates JWT for user
func (m *Manager) GenerateToken(user User) (string, error) {
	expiresAt := time.Now().Add(m.config.TokenExpiration)
	if m.config.TokenExpiration == 0 {
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	claims := &Claims{
		User: user,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "governance-sidecar",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ValidateToken verifies JWT and returns user
func (m *Manager) ValidateToken(tokenString string) (*User, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return &claims.User, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// GetUserFromContext extracts user from Echo context
func GetUserFromContext(c echo.Context) *User {
	if user, ok := c.Get("user").(*User); ok {
		return user
	}
	return nil
}

// GetUserFromStdContext extracts user from standard context
func GetUserFromStdContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value("user").(*User)
	return user, ok
}

// hasRequiredRole checks if user has required role
func (m *Manager) hasRequiredRole(user *User) bool {
	for _, required := range m.config.AllowedRoles {
		for _, userRole := range user.Roles {
			if userRole == required {
				return true
			}
		}
	}
	return false
}

// Role constants
const (
	RoleAdmin    = "admin"
	RoleApprover = "approver"
	RoleViewer   = "viewer"
)