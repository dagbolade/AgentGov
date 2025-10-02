package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestMiddlewareAuthDisabled(t *testing.T) {
	manager := NewManager(Config{
		JWTSecret:   "test-secret",
		RequireAuth: false, // Auth disabled
	})
	
	e := echo.New()
	
	// Setup route with auth middleware
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	}, manager.Middleware())
	
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	
	e.ServeHTTP(rec, req)
	
	// Should pass through without token
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

func TestMiddlewarePublicEndpoints(t *testing.T) {
	manager := NewManager(Config{
		JWTSecret:   "test-secret",
		RequireAuth: true,
	})
	
	e := echo.New()
	e.Use(manager.Middleware())
	
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "healthy")
	})
	
	e.POST("/login", func(c echo.Context) error {
		return c.String(http.StatusOK, "login")
	})
	
	// Test /health endpoint
	req1 := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)
	
	// Test /login endpoint
	req2 := httptest.NewRequest(http.MethodPost, "/login", nil)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
}

func TestMiddlewareMissingToken(t *testing.T) {
	manager := NewManager(Config{
		JWTSecret:   "test-secret",
		RequireAuth: true,
	})
	
	e := echo.New()
	e.Use(manager.Middleware())
	
	e.GET("/protected", func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})
	
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Missing authorization header")
}

func TestMiddlewareInvalidTokenFormat(t *testing.T) {
	manager := NewManager(Config{
		JWTSecret:   "test-secret",
		RequireAuth: true,
	})
	
	e := echo.New()
	e.Use(manager.Middleware())
	
	e.GET("/protected", func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})
	
	tests := []struct {
		name   string
		header string
	}{
		{"missing bearer", "just-a-token"},
		{"wrong prefix", "Basic token123"},
		{"empty token", "Bearer "},
		{"extra spaces", "Bearer  token  extra"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()
			
			e.ServeHTTP(rec, req)
			
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

func TestMiddlewareValidToken(t *testing.T) {
	manager := NewManager(Config{
		JWTSecret:   "test-secret",
		RequireAuth: true,
	})
	
	// Generate valid token
	user := User{
		ID:    "test-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{RoleAdmin},
	}
	
	token, err := manager.GenerateToken(user)
	assert.NoError(t, err)
	
	e := echo.New()
	e.Use(manager.Middleware())
	
	e.GET("/protected", func(c echo.Context) error {
		// Check user is in context
		contextUser := GetUserFromContext(c)
		assert.NotNil(t, contextUser)
		assert.Equal(t, user.Email, contextUser.Email)
		return c.String(http.StatusOK, "success")
	})
	
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "success", rec.Body.String())
}

func TestMiddlewareExpiredToken(t *testing.T) {
	manager := NewManager(Config{
		JWTSecret:       "test-secret",
		TokenExpiration: -1 * time.Hour, // Expired
		RequireAuth:     true,
	})
	
	user := User{
		ID:    "test-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{RoleAdmin},
	}
	
	token, err := manager.GenerateToken(user)
	assert.NoError(t, err)
	
	e := echo.New()
	e.Use(manager.Middleware())
	
	e.GET("/protected", func(c echo.Context) error {
		return c.String(http.StatusOK, "success")
	})
	
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	
	e.ServeHTTP(rec, req)
	
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid token")
}

func TestRequireRoleMiddleware(t *testing.T) {
	manager := NewManager(Config{
		JWTSecret:   "test-secret",
		RequireAuth: true,
	})
	
	e := echo.New()
	e.Use(manager.Middleware())
	
	// Route requiring admin role
	e.GET("/admin-only", func(c echo.Context) error {
		return c.String(http.StatusOK, "admin access")
	}, manager.RequireRole(RoleAdmin))
	
	// Test with admin user
	adminUser := User{
		ID:    "admin-123",
		Email: "admin@example.com",
		Name:  "Admin User",
		Roles: []string{RoleAdmin, RoleApprover},
	}
	adminToken, _ := manager.GenerateToken(adminUser)
	
	req1 := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req1.Header.Set("Authorization", "Bearer "+adminToken)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)
	
	assert.Equal(t, http.StatusOK, rec1.Code)
	
	// Test with viewer user (no admin role)
	viewerUser := User{
		ID:    "viewer-123",
		Email: "viewer@example.com",
		Name:  "Viewer User",
		Roles: []string{RoleViewer},
	}
	viewerToken, _ := manager.GenerateToken(viewerUser)
	
	req2 := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req2.Header.Set("Authorization", "Bearer "+viewerToken)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	
	assert.Equal(t, http.StatusForbidden, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "Role 'admin' required")
}

func TestGenerateAndValidateToken(t *testing.T) {
	manager := NewManager(Config{
		JWTSecret:       "test-secret-key",
		TokenExpiration: 1 * time.Hour,
	})
	
	user := User{
		ID:    "user-123",
		Email: "user@example.com",
		Name:  "Test User",
		Roles: []string{RoleApprover, RoleViewer},
	}
	
	// Generate token
	token, err := manager.GenerateToken(user)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	
	// Validate token
	validatedUser, err := manager.ValidateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, validatedUser.ID)
	assert.Equal(t, user.Email, validatedUser.Email)
	assert.Equal(t, user.Name, validatedUser.Name)
	assert.Equal(t, user.Roles, validatedUser.Roles)
}

func TestTokenWithDifferentSecret(t *testing.T) {
	manager1 := NewManager(Config{
		JWTSecret: "secret-1",
	})
	
	manager2 := NewManager(Config{
		JWTSecret: "secret-2",
	})
	
	user := User{
		ID:    "test-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{RoleAdmin},
	}
	
	// Generate token with manager1
	token, err := manager1.GenerateToken(user)
	assert.NoError(t, err)
	
	// Try to validate with manager2 (different secret)
	_, err = manager2.ValidateToken(token)
	assert.Error(t, err)
}

func TestGetUserFromContext(t *testing.T) {
	e := echo.New()
	
	user := &User{
		ID:    "test-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{RoleAdmin},
	}
	
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	// Test with user in context
	c.Set("user", user)
	retrievedUser := GetUserFromContext(c)
	assert.NotNil(t, retrievedUser)
	assert.Equal(t, user.Email, retrievedUser.Email)
	
	// Test without user in context
	c2 := e.NewContext(req, rec)
	retrievedUser2 := GetUserFromContext(c2)
	assert.Nil(t, retrievedUser2)
}

func TestHasRequiredRole(t *testing.T) {
	manager := NewManager(Config{
		JWTSecret:    "test-secret",
		AllowedRoles: []string{RoleAdmin, RoleApprover},
	})
	
	tests := []struct {
		name     string
		userRole []string
		expected bool
	}{
		{"admin has access", []string{RoleAdmin}, true},
		{"approver has access", []string{RoleApprover}, true},
		{"viewer no access", []string{RoleViewer}, false},
		{"multiple roles match", []string{RoleViewer, RoleApprover}, true},
		{"no roles", []string{}, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{Roles: tt.userRole}
			result := manager.hasRequiredRole(user)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRoleConstants(t *testing.T) {
	assert.Equal(t, "admin", RoleAdmin)
	assert.Equal(t, "approver", RoleApprover)
	assert.Equal(t, "viewer", RoleViewer)
}