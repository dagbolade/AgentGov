package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func setupTestAuth() (*Manager, *Handler, *echo.Echo) {
	manager := NewManager(Config{
		JWTSecret:       "test-secret-key",
		TokenExpiration: 24 * time.Hour,
		RequireAuth:     true,
	})
	
	handler := NewHandler(manager)
	e := echo.New()
	
	return manager, handler, e
}

func TestLoginSuccess(t *testing.T) {
	_, handler, e := setupTestAuth()
	
	// Set test user in environment
	t.Setenv("AUTH_USERS", "test@example.com:password123:Test User:admin,approver")
	
	body := `{"email":"test@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := handler.Login(c)
	
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	
	// Verify response contains token and user
	assert.Contains(t, rec.Body.String(), "token")
	assert.Contains(t, rec.Body.String(), "test@example.com")
	assert.Contains(t, rec.Body.String(), "Test User")
}

func TestLoginInvalidCredentials(t *testing.T) {
	_, handler, e := setupTestAuth()
	
	t.Setenv("AUTH_USERS", "test@example.com:password123:Test:admin")
	
	body := `{"email":"test@example.com","password":"wrongpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := handler.Login(c)
	
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "Invalid credentials")
}

func TestLoginMissingEmail(t *testing.T) {
	_, handler, e := setupTestAuth()
	
	body := `{"password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := handler.Login(c)
	
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestLoginInvalidJSON(t *testing.T) {
	_, handler, e := setupTestAuth()
	
	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := handler.Login(c)
	
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestLoginDefaultCredentials(t *testing.T) {
	_, handler, e := setupTestAuth()
	
	// Don't set AUTH_USERS, should use default
	
	body := `{"email":"admin@example.com","password":"admin"}`
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	err := handler.Login(c)
	
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestLoginMultipleUsers(t *testing.T) {
	_, handler, e := setupTestAuth()
	
	t.Setenv("AUTH_USERS", "user1@test.com:pass1:User One:admin;user2@test.com:pass2:User Two:approver")
	
	// Test first user
	body1 := `{"email":"user1@test.com","password":"pass1"}`
	req1 := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body1))
	req1.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)
	
	err1 := handler.Login(c1)
	assert.NoError(t, err1)
	assert.Equal(t, http.StatusOK, rec1.Code)
	assert.Contains(t, rec1.Body.String(), "User One")
	
	// Test second user
	body2 := `{"email":"user2@test.com","password":"pass2"}`
	req2 := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body2))
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	
	err2 := handler.Login(c2)
	assert.NoError(t, err2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "User Two")
}

func TestMeEndpoint(t *testing.T) {
	_, handler, e := setupTestAuth()
	
	// Create test user
	user := User{
		ID:    "test-123",
		Email: "test@example.com",
		Name:  "Test User",
		Roles: []string{RoleAdmin},
	}
	
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	// Set user in context
	c.Set("user", &user)
	
	err := handler.Me(c)
	
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "test@example.com")
	assert.Contains(t, rec.Body.String(), "Test User")
}

func TestMeEndpointUnauthorized(t *testing.T) {
	_, handler, e := setupTestAuth()
	
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	
	// Don't set user in context
	
	err := handler.Me(c)
	
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestValidateCredentialsTimingAttack(t *testing.T) {
	_, handler, _ := setupTestAuth()
	
	t.Setenv("AUTH_USERS", "test@example.com:password123:Test:admin")
	
	// Both should take similar time (constant-time comparison)
	start1 := time.Now()
	_, err1 := handler.validateCredentials("test@example.com", "wrongpassword")
	duration1 := time.Since(start1)
	
	start2 := time.Now()
	_, err2 := handler.validateCredentials("wrong@example.com", "password123")
	duration2 := time.Since(start2)
	
	assert.Error(t, err1)
	assert.Error(t, err2)
	
	// Durations should be similar (within 10ms)
	diff := duration1 - duration2
	if diff < 0 {
		diff = -diff
	}
	assert.Less(t, diff, 10*time.Millisecond, "Timing difference too large, possible timing attack vulnerability")
}

func TestGenerateUserID(t *testing.T) {
	tests := []struct {
		email    string
		expected string
	}{
		{"test@example.com", "test-example.com"},
		{"admin@company.org", "admin-company.org"},
		{"user.name@domain.co.uk", "user.name-domain.co.uk"},
	}
	
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := generateUserID(tt.email)
			assert.Equal(t, tt.expected, result)
		})
	}
}