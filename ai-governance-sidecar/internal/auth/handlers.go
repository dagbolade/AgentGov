package auth

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Handler provides HTTP handlers for auth
type Handler struct {
	manager *Manager
}

// NewHandler creates auth handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// LoginRequest represents login credentials
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse contains JWT token
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// Login handles authentication
func (h *Handler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		log.Warn().Err(err).Str("remote_addr", c.Request().RemoteAddr).Msg("invalid login request body")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request",
		})
	}

	// Validate credentials
	user, err := h.validateCredentials(req.Email, req.Password)
	if err != nil {
		log.Warn().Str("email", req.Email).Msg("login failed")
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Invalid credentials",
		})
	}

	// Generate token
	token, err := h.manager.GenerateToken(*user)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate token")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to generate token",
		})
	}

	log.Info().Str("email", user.Email).Msg("user logged in")

	return c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  *user,
	})
}

// Me returns current user info
func (h *Handler) Me(c echo.Context) error {
	user := GetUserFromContext(c)
	if user == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Unauthorized",
		})
	}

	return c.JSON(http.StatusOK, user)
}

// validateCredentials checks user credentials
// Format: EMAIL:PASSWORD:NAME:ROLES (semicolon-separated users)
// Example: admin@example.com:pass123:Admin:admin,approver
func (h *Handler) validateCredentials(email, password string) (*User, error) {
	usersEnv := os.Getenv("AUTH_USERS")
	if usersEnv == "" {
		// Default admin user for development
		usersEnv = "admin@example.com:admin:Administrator:admin,approver"
	}

	// Parse users
	users := strings.Split(usersEnv, ";")
	for _, userStr := range users {
		parts := strings.Split(userStr, ":")
		if len(parts) < 4 {
			continue
		}

		userEmail := parts[0]
		userPassword := parts[1]
		userName := parts[2]
		rolesStr := parts[3]

		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(email), []byte(userEmail)) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte(userPassword)) == 1 {

			roles := strings.Split(rolesStr, ",")
			return &User{
				ID:    generateUserID(email),
				Email: email,
				Name:  userName,
				Roles: roles,
			}, nil
		}
	}

	return nil, ErrInvalidCredentials
}

// generateUserID creates consistent ID from email
func generateUserID(email string) string {
	return strings.ReplaceAll(email, "@", "-")
}

// Error types
var (
	ErrInvalidCredentials = &AuthError{"Invalid credentials"}
)

// AuthError represents authentication error
type AuthError struct {
	message string
}

func (e *AuthError) Error() string {
	return e.message
}
