package web_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"game-api/internal/di"
	"game-api/internal/domain"
	"game-api/internal/web"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type MockUserService struct {
	users map[string]domain.User
}

func (m *MockUserService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return &u, nil
}

func (m *MockUserService) GetUserByLogin(ctx context.Context, login string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Login == login {
			return &u, nil
		}
	}
	return nil, nil
}

func (m *MockUserService) SaveUser(ctx context.Context, u domain.User) error {
	if m.users == nil {
		m.users = make(map[string]domain.User)
	}
	m.users[u.UUID] = u
	return nil
}

type dummyHandler struct{}

func (h *dummyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userUUID := r.Context().Value(web.UserUUIDKey)
	if userUUID == nil {
		http.Error(w, "no user in context", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func TestAuthFlow(t *testing.T) {
	ctx := context.Background()
	userSvc := &MockUserService{users: make(map[string]domain.User)}
	authSvc := &domain.AuthService{UserSvc: userSvc}
	handler := web.NewUserHandler(authSvc)
	authenticator := web.NewUserAuthenticator(authSvc)

	// 1. Register user
	signUpPayload := domain.SignUpRequest{
		Login:    "testuser",
		Password: "password123",
	}
	signUpJSON, _ := json.Marshal(signUpPayload)

	req := httptest.NewRequest(http.MethodPost, "/signup", bytes.NewBuffer(signUpJSON))
	rec := httptest.NewRecorder()
	handler.RegisterUser(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected status 201 Created on registration, got %d", rec.Code)
	}

	// Verify user is in db
	savedUser, err := userSvc.GetUserByLogin(ctx, "testuser")
	if err != nil {
		t.Fatalf("Failed to fetch user by login: %v", err)
	}
	if savedUser == nil {
		t.Fatalf("User was not saved to database")
	}

	err = bcrypt.CompareHashAndPassword([]byte(savedUser.PasswordHash), []byte("password123"))
	if err != nil {
		t.Fatalf("Saved password hash does not match original password: %v", err)
	}

	// 2. Register duplicate user
	req = httptest.NewRequest(http.MethodPost, "/signup", bytes.NewBuffer(signUpJSON))
	rec = httptest.NewRecorder()
	handler.RegisterUser(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request on duplicate registration, got %d", rec.Code)
	}

	// 3. Login with correct credentials
	creds := base64.StdEncoding.EncodeToString([]byte("testuser:password123"))
	req = httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("Authorization", "Basic "+creds)
	rec = httptest.NewRecorder()
	handler.AuthUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK on correct login, got %d", rec.Code)
	}

	var loginResp struct {
		UUID string `json:"uuid"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&loginResp); err != nil {
		t.Fatalf("Failed to decode login response: %v", err)
	}

	parsedUUID, err := uuid.Parse(loginResp.UUID)
	if err != nil {
		t.Fatalf("Response UUID is invalid: %v", err)
	}
	if parsedUUID.String() != savedUser.UUID {
		t.Errorf("Response UUID %s does not match saved UUID %s", parsedUUID.String(), savedUser.UUID)
	}

	// 4. Login with incorrect password
	badCreds := base64.StdEncoding.EncodeToString([]byte("testuser:wrongpassword"))
	req = httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("Authorization", "Basic "+badCreds)
	rec = httptest.NewRecorder()
	handler.AuthUser(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized on incorrect password, got %d", rec.Code)
	}

	// 5. Login with non-existing user
	nonExistingCreds := base64.StdEncoding.EncodeToString([]byte("nonexistent:password"))
	req = httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("Authorization", "Basic "+nonExistingCreds)
	rec = httptest.NewRecorder()
	handler.AuthUser(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized on non-existing user, got %d", rec.Code)
	}

	// 6. Test Middleware with valid auth
	protectedHandler := authenticator.Middleware(&dummyHandler{})
	req = httptest.NewRequest(http.MethodPost, "/game/test-uuid", nil)
	req.Header.Set("Authorization", "Basic "+creds)
	rec = httptest.NewRecorder()
	protectedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected middleware to allow request with correct auth, got status %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("Expected response body to be 'ok', got '%s'", rec.Body.String())
	}

	// 7. Test Middleware with invalid auth
	req = httptest.NewRequest(http.MethodPost, "/game/test-uuid", nil)
	req.Header.Set("Authorization", "Basic "+badCreds)
	rec = httptest.NewRecorder()
	protectedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected middleware to block request with incorrect auth, got status %d", rec.Code)
	}

	// 8. Test Middleware with missing auth
	req = httptest.NewRequest(http.MethodPost, "/game/test-uuid", nil)
	rec = httptest.NewRecorder()
	protectedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected middleware to block request with missing auth, got status %d", rec.Code)
	}

	// 9. Test GetUser handler success
	req = httptest.NewRequest(http.MethodGet, "/user/"+savedUser.UUID, nil)
	req.SetPathValue("uuid", savedUser.UUID)
	rec = httptest.NewRecorder()
	handler.GetUser(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Expected GetUser 200 OK, got status %d", rec.Code)
	}
	var uResp web.UserResponse
	json.NewDecoder(rec.Body).Decode(&uResp)
	if uResp.Login != "testuser" || uResp.UUID != savedUser.UUID {
		t.Errorf("GetUser returned unexpected user: %+v", uResp)
	}

	// 10. GetUser with empty uuid
	req = httptest.NewRequest(http.MethodGet, "/user/", nil)
	rec = httptest.NewRecorder()
	handler.GetUser(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected GetUser 400 Bad Request on empty UUID, got status %d", rec.Code)
	}

	// 11. GetUser with non-existent uuid
	req = httptest.NewRequest(http.MethodGet, "/user/non-existent", nil)
	req.SetPathValue("uuid", "non-existent")
	rec = httptest.NewRecorder()
	handler.GetUser(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected GetUser 404 Not Found, got status %d", rec.Code)
	}
}

func TestAuthEdgeCases(t *testing.T) {
	userSvc := &MockUserService{users: make(map[string]domain.User)}
	authSvc := &domain.AuthService{UserSvc: userSvc}
	handler := web.NewUserHandler(authSvc)

	// 1. Malformed JSON body in signup
	req := httptest.NewRequest(http.MethodPost, "/signup", bytes.NewBuffer([]byte(`{"login": "testuser",`)))
	rec := httptest.NewRecorder()
	handler.RegisterUser(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request on malformed JSON body in signup, got %d", rec.Code)
	}

	// 2. Invalid Base64 in Authorization header
	req = httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("Authorization", "Basic invalid-base64-!!!")
	rec = httptest.NewRecorder()
	handler.AuthUser(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized on invalid Base64 in Auth header, got %d", rec.Code)
	}

	// 3. Invalid Auth Scheme (e.g. Bearer)
	req = httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("Authorization", "Bearer someToken123")
	rec = httptest.NewRecorder()
	handler.AuthUser(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized on non-Basic Auth Scheme, got %d", rec.Code)
	}

	// 4. Invalid credentials format (no colon inside decoded credentials)
	badEncoding := base64.StdEncoding.EncodeToString([]byte("userwithoutcolon"))
	req = httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("Authorization", "Basic "+badEncoding)
	rec = httptest.NewRecorder()
	handler.AuthUser(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized on credentials format with no colon, got %d", rec.Code)
	}

	// 5. Test routing with wrong HTTP Method
	mux := di.NewServeMux()
	gameHandler := web.NewGameHandler(nil)
	authenticator := web.NewUserAuthenticator(authSvc)
	di.RegisterRoute(mux, gameHandler, handler, authenticator)

	req = httptest.NewRequest(http.MethodGet, "/signup", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 Method Not Allowed on GET /signup, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/login", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 Method Not Allowed on GET /login, got %d", rec.Code)
	}
}
