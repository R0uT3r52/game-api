package web_test

import (
	"bytes"
	"context"
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

func setupWebJwtProvider(t *testing.T) domain.JwtProviderInterface {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-web-key-12345")
	jwtProvider, err := domain.NewJwtProvider()
	if err != nil {
		t.Fatalf("Failed to create JwtProvider: %v", err)
	}
	return jwtProvider
}

func TestAuthFlow(t *testing.T) {
	ctx := context.Background()
	userSvc := &MockUserService{users: make(map[string]domain.User)}
	jwtProvider := setupWebJwtProvider(t)
	authSvc := &domain.AuthService{UserSvc: userSvc, JwtProvider: jwtProvider}
	handler := web.NewUserHandler(authSvc)
	authenticator := web.NewUserAuthenticator(jwtProvider)

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

	// 3. Login with correct credentials (JWT login)
	loginPayload := domain.JwtRequest{
		Login:    "testuser",
		Password: "password123",
	}
	loginJSON, _ := json.Marshal(loginPayload)
	req = httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(loginJSON))
	rec = httptest.NewRecorder()
	handler.AuthUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK on correct login, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	var jwtResp domain.JwtResponse
	if err := json.NewDecoder(rec.Body).Decode(&jwtResp); err != nil {
		t.Fatalf("Failed to decode jwt login response: %v", err)
	}

	if jwtResp.Type != "Bearer" {
		t.Errorf("Expected token type 'Bearer', got '%s'", jwtResp.Type)
	}
	if jwtResp.AccessToken == "" || jwtResp.RefreshToken == "" {
		t.Fatalf("Expected non-empty AccessToken and RefreshToken in login response")
	}

	// Verify UUID in access token matches saved user
	uuidFromToken, err := jwtProvider.GetUUIDFromToken(jwtResp.AccessToken)
	if err != nil {
		t.Fatalf("Failed to parse UUID from access token: %v", err)
	}
	parsedUUID, err := uuid.Parse(uuidFromToken)
	if err != nil {
		t.Fatalf("Extracted UUID is invalid: %v", err)
	}
	if parsedUUID.String() != savedUser.UUID {
		t.Errorf("Token UUID %s does not match saved UUID %s", parsedUUID.String(), savedUser.UUID)
	}

	// 4. Login with incorrect password
	badLoginPayload := domain.JwtRequest{
		Login:    "testuser",
		Password: "wrongpassword",
	}
	badLoginJSON, _ := json.Marshal(badLoginPayload)
	req = httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(badLoginJSON))
	rec = httptest.NewRecorder()
	handler.AuthUser(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized on incorrect password, got %d", rec.Code)
	}

	// 5. Login with non-existing user
	nonExistingPayload := domain.JwtRequest{
		Login:    "nonexistent",
		Password: "password",
	}
	nonExistingJSON, _ := json.Marshal(nonExistingPayload)
	req = httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(nonExistingJSON))
	rec = httptest.NewRecorder()
	handler.AuthUser(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized on non-existing user, got %d", rec.Code)
	}

	// 6. Test Middleware with valid Bearer auth
	protectedHandler := authenticator.Middleware(&dummyHandler{})
	req = httptest.NewRequest(http.MethodPost, "/game/test-uuid", nil)
	req.Header.Set("Authorization", "Bearer "+jwtResp.AccessToken)
	rec = httptest.NewRecorder()
	protectedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected middleware to allow request with valid Bearer auth, got status %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("Expected response body to be 'ok', got '%s'", rec.Body.String())
	}

	// 7. Test Middleware with invalid Bearer token
	req = httptest.NewRequest(http.MethodPost, "/game/test-uuid", nil)
	req.Header.Set("Authorization", "Bearer invalid-tampered-token")
	rec = httptest.NewRecorder()
	protectedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected middleware to block request with invalid Bearer token, got status %d", rec.Code)
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

	// 12. Test GetUserByToken (GET /user with Bearer token)
	getUserByTokenHandler := authenticator.Middleware(http.HandlerFunc(handler.GetUserByToken))
	req = httptest.NewRequest(http.MethodGet, "/user", nil)
	req.Header.Set("Authorization", "Bearer "+jwtResp.AccessToken)
	rec = httptest.NewRecorder()
	getUserByTokenHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected GetUserByToken 200 OK, got %d", rec.Code)
	}
	var tokenUserResp web.UserResponse
	if err := json.NewDecoder(rec.Body).Decode(&tokenUserResp); err != nil {
		t.Fatalf("Failed to decode GetUserByToken response: %v", err)
	}
	if tokenUserResp.UUID != savedUser.UUID || tokenUserResp.Login != savedUser.Login {
		t.Errorf("GetUserByToken returned %+v, expected UUID %s and Login %s", tokenUserResp, savedUser.UUID, savedUser.Login)
	}

	// 13. Test UpdateAccessToken (/refresh-acc)
	refPayload := web.RefreshJwtRequest{RefreshToken: jwtResp.RefreshToken}
	refJSON, _ := json.Marshal(refPayload)
	req = httptest.NewRequest(http.MethodPost, "/refresh-acc", bytes.NewBuffer(refJSON))
	rec = httptest.NewRecorder()
	handler.UpdateAccessToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected UpdateAccessToken 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var refreshedAccResp domain.JwtResponse
	if err := json.NewDecoder(rec.Body).Decode(&refreshedAccResp); err != nil {
		t.Fatalf("Failed to decode UpdateAccessToken response: %v", err)
	}
	if refreshedAccResp.AccessToken == "" {
		t.Errorf("Expected new non-empty AccessToken")
	}
	if refreshedAccResp.RefreshToken != jwtResp.RefreshToken {
		t.Errorf("Expected preserved RefreshToken %s, got %s", jwtResp.RefreshToken, refreshedAccResp.RefreshToken)
	}

	// 14. Test UpdateRefreshToken (/refresh-ref)
	req = httptest.NewRequest(http.MethodPost, "/refresh-ref", bytes.NewBuffer(refJSON))
	rec = httptest.NewRecorder()
	handler.UpdateRefreshToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected UpdateRefreshToken 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	var refreshedRefResp domain.JwtResponse
	if err := json.NewDecoder(rec.Body).Decode(&refreshedRefResp); err != nil {
		t.Fatalf("Failed to decode UpdateRefreshToken response: %v", err)
	}
	if refreshedRefResp.AccessToken == "" {
		t.Errorf("Expected new non-empty AccessToken")
	}
	if refreshedRefResp.RefreshToken == "" {
		t.Errorf("Expected new non-empty RefreshToken")
	}
}

func TestAuthEdgeCases(t *testing.T) {
	userSvc := &MockUserService{users: make(map[string]domain.User)}
	jwtProvider := setupWebJwtProvider(t)
	authSvc := &domain.AuthService{UserSvc: userSvc, JwtProvider: jwtProvider}
	handler := web.NewUserHandler(authSvc)
	authenticator := web.NewUserAuthenticator(jwtProvider)

	// 1. Malformed JSON body in signup
	req := httptest.NewRequest(http.MethodPost, "/signup", bytes.NewBuffer([]byte(`{"login": "testuser",`)))
	rec := httptest.NewRecorder()
	handler.RegisterUser(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request on malformed JSON body in signup, got %d", rec.Code)
	}

	// 2. Malformed JSON body in login
	req = httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer([]byte(`{"login": "testuser"`)))
	rec = httptest.NewRecorder()
	handler.AuthUser(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request on malformed JSON body in login, got %d", rec.Code)
	}

	// 3. Non-Bearer authorization header in Middleware
	protectedHandler := authenticator.Middleware(&dummyHandler{})
	req = httptest.NewRequest(http.MethodPost, "/game/test-uuid", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec = httptest.NewRecorder()
	protectedHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized on non-Bearer Authorization scheme, got %d", rec.Code)
	}

	// 4. Malformed Authorization header in Middleware
	req = httptest.NewRequest(http.MethodPost, "/game/test-uuid", nil)
	req.Header.Set("Authorization", "Bearer")
	rec = httptest.NewRecorder()
	protectedHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized on empty Bearer token, got %d", rec.Code)
	}

	// 5. UpdateAccessToken with malformed JSON body
	req = httptest.NewRequest(http.MethodPost, "/refresh-acc", bytes.NewBuffer([]byte(`{"refresh_token":`)))
	rec = httptest.NewRecorder()
	handler.UpdateAccessToken(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request on malformed JSON in UpdateAccessToken, got %d", rec.Code)
	}

	// 6. UpdateAccessToken with invalid refresh token
	invRefReq, _ := json.Marshal(web.RefreshJwtRequest{RefreshToken: "invalid-token"})
	req = httptest.NewRequest(http.MethodPost, "/refresh-acc", bytes.NewBuffer(invRefReq))
	rec = httptest.NewRecorder()
	handler.UpdateAccessToken(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized on invalid token in UpdateAccessToken, got %d", rec.Code)
	}

	// 7. UpdateRefreshToken with malformed JSON body
	req = httptest.NewRequest(http.MethodPost, "/refresh-ref", bytes.NewBuffer([]byte(`{"refresh_token":`)))
	rec = httptest.NewRecorder()
	handler.UpdateRefreshToken(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request on malformed JSON in UpdateRefreshToken, got %d", rec.Code)
	}

	// 8. UpdateRefreshToken with invalid refresh token
	req = httptest.NewRequest(http.MethodPost, "/refresh-ref", bytes.NewBuffer(invRefReq))
	rec = httptest.NewRecorder()
	handler.UpdateRefreshToken(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized on invalid token in UpdateRefreshToken, got %d", rec.Code)
	}

	// 9. GetUserByToken with missing UUID in context
	req = httptest.NewRequest(http.MethodGet, "/user", nil)
	rec = httptest.NewRecorder()
	handler.GetUserByToken(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 on missing UUID context in GetUserByToken, got %d", rec.Code)
	}

	// 10. Test routing with wrong HTTP Method
	mux := di.NewServeMux()
	gameHandler := web.NewGameHandler(nil)
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

	req = httptest.NewRequest(http.MethodGet, "/refresh-acc", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 Method Not Allowed on GET /refresh-acc, got %d", rec.Code)
	}
}
