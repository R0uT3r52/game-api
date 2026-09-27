package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type MockUserSvc struct {
	users map[string]User
}

func (m *MockUserSvc) GetUser(ctx context.Context, uuid string) (*User, error) {
	u, ok := m.users[uuid]
	if !ok {
		return nil, nil
	}
	return &u, nil
}

func (m *MockUserSvc) GetUserByLogin(ctx context.Context, login string) (*User, error) {
	for _, u := range m.users {
		if u.Login == login {
			return &u, nil
		}
	}
	return nil, nil
}

func (m *MockUserSvc) SaveUser(ctx context.Context, u User) error {
	if m.users == nil {
		m.users = make(map[string]User)
	}
	m.users[u.UUID] = u
	return nil
}

func setupTestJwtProvider(t *testing.T) JwtProviderInterface {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-domain-key-12345")
	jwtProvider, err := NewJwtProvider()
	if err != nil {
		t.Fatalf("Failed to create JwtProvider: %v", err)
	}
	return jwtProvider
}

func TestAuthService_RegisterAndAuthorize(t *testing.T) {
	ctx := context.Background()
	userSvc := &MockUserSvc{users: make(map[string]User)}
	jwtProvider := setupTestJwtProvider(t)
	authSvc := &AuthService{UserSvc: userSvc, JwtProvider: jwtProvider}

	// 1. Success Register
	req := SignUpRequest{
		Login:    "alice",
		Password: "alicepassword",
	}

	err := authSvc.Register(ctx, req)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 2. Duplicate Register
	err = authSvc.Register(ctx, req)
	var duplicateErr *UserAlreadyExistsError
	if !errors.As(err, &duplicateErr) {
		t.Errorf("Expected UserAlreadyExistsError, got %v", err)
	}

	// 3. Success Authorize
	jwtRep, err := authSvc.Authorize(ctx, &JwtRequest{
		Login:    "alice",
		Password: "alicepassword",
	})
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}
	if jwtRep == nil {
		t.Fatalf("Expected non-nil JwtResponse")
	}
	if jwtRep.Type != "Bearer" {
		t.Errorf("Expected token type 'Bearer', got '%s'", jwtRep.Type)
	}
	if jwtRep.AccessToken == "" {
		t.Errorf("Expected non-empty AccessToken")
	}
	if jwtRep.RefreshToken == "" {
		t.Errorf("Expected non-empty RefreshToken")
	}

	// 4. Verify token claims and extract UUID
	uuidFromToken, err := jwtProvider.GetUUIDFromToken(jwtRep.AccessToken)
	if err != nil {
		t.Fatalf("Failed to extract UUID from access token: %v", err)
	}
	if uuidFromToken == "" {
		t.Fatalf("Extracted UUID is empty")
	}

	// 5. Wrong password
	_, err = authSvc.Authorize(ctx, &JwtRequest{
		Login:    "alice",
		Password: "wrongpassword",
	})
	var incorrectCredsErr *IncorrectCredsError
	if !errors.As(err, &incorrectCredsErr) {
		t.Errorf("Expected IncorrectCredsError, got %v", err)
	}

	// 6. Non-existent user
	_, err = authSvc.Authorize(ctx, &JwtRequest{
		Login:    "bob",
		Password: "bobpassword",
	})
	if !errors.As(err, &incorrectCredsErr) {
		t.Errorf("Expected IncorrectCredsError for non-existent user, got %v", err)
	}

	// 7. GetUser using extracted UUID
	user, err := authSvc.GetUser(ctx, uuidFromToken)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if user == nil || user.Login != "alice" {
		t.Errorf("GetUser returned wrong user: %v", user)
	}

	// 8. Empty Register
	req2 := SignUpRequest{
		Login:    "",
		Password: "",
	}

	err2 := authSvc.Register(ctx, req2)
	var validationErr *ValidationError
	if !errors.As(err2, &validationErr) {
		t.Errorf("Expected ValidationError on empty signup, got %v", err2)
	}
}

func TestJwtProvider(t *testing.T) {
	// Test missing secret
	t.Run("Missing JWT_SECRET", func(t *testing.T) {
		t.Setenv("JWT_SECRET", "")
		_, err := NewJwtProvider()
		var secretErr *JwtSecretError
		if !errors.As(err, &secretErr) {
			t.Errorf("Expected JwtSecretError, got %v", err)
		}
	})

	jwtProvider := setupTestJwtProvider(t)
	user := &User{
		UUID:  "11111111-2222-3333-4444-555555555555",
		Login: "jwtuser",
	}

	t.Run("Generate and validate access token", func(t *testing.T) {
		token, err := jwtProvider.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("GenerateAccessToken failed: %v", err)
		}

		err = jwtProvider.ValidateAccessToken(token)
		if err != nil {
			t.Errorf("ValidateAccessToken failed on valid token: %v", err)
		}

		// Valid access token should fail refresh token validation
		err = jwtProvider.ValidateRefreshToken(token)
		var invalidErr *InvalidTokenError
		if !errors.As(err, &invalidErr) {
			t.Errorf("Expected InvalidTokenError when validating access token as refresh token, got %v", err)
		}

		extractedUUID, err := jwtProvider.GetUUIDFromToken(token)
		if err != nil {
			t.Fatalf("GetUUIDFromToken failed: %v", err)
		}
		if extractedUUID != user.UUID {
			t.Errorf("Expected UUID %s, got %s", user.UUID, extractedUUID)
		}
	})

	t.Run("Generate and validate refresh token", func(t *testing.T) {
		token, err := jwtProvider.GenerateRefreshToken(user)
		if err != nil {
			t.Fatalf("GenerateRefreshToken failed: %v", err)
		}

		err = jwtProvider.ValidateRefreshToken(token)
		if err != nil {
			t.Errorf("ValidateRefreshToken failed on valid token: %v", err)
		}

		// Valid refresh token should fail access token validation
		err = jwtProvider.ValidateAccessToken(token)
		var invalidErr *InvalidTokenError
		if !errors.As(err, &invalidErr) {
			t.Errorf("Expected InvalidTokenError when validating refresh token as access token, got %v", err)
		}

		extractedUUID, err := jwtProvider.GetUUIDFromToken(token)
		if err != nil {
			t.Fatalf("GetUUIDFromToken failed: %v", err)
		}
		if extractedUUID != user.UUID {
			t.Errorf("Expected UUID %s, got %s", user.UUID, extractedUUID)
		}
	})

	t.Run("Invalid and tampered tokens", func(t *testing.T) {
		validToken, err := jwtProvider.GenerateAccessToken(user)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		tamperedToken := validToken + "tampered"
		if err := jwtProvider.ValidateAccessToken(tamperedToken); err == nil {
			t.Errorf("Expected error for tampered access token")
		}
		if err := jwtProvider.ValidateRefreshToken(tamperedToken); err == nil {
			t.Errorf("Expected error for tampered refresh token")
		}
		if _, err := jwtProvider.GetUUIDFromToken(tamperedToken); err == nil {
			t.Errorf("Expected error extracting UUID from tampered token")
		}

		// Token signed with different secret
		diffClaims := jwt.MapClaims{
			"uuid":       user.UUID,
			"exp":        time.Now().Add(time.Hour).Unix(),
			"token_type": "access",
		}
		diffTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, diffClaims)
		diffToken, _ := diffTokenObj.SignedString([]byte("different-secret"))

		if err := jwtProvider.ValidateAccessToken(diffToken); err == nil {
			t.Errorf("Expected error for token signed with different secret")
		}
		if _, err := jwtProvider.GetUUIDFromToken(diffToken); err == nil {
			t.Errorf("Expected error extracting UUID from token with different secret")
		}

		// Malformed token strings
		malformedTokens := []string{"", "not-a-token", "a.b", "a.b.c.d"}
		for _, mt := range malformedTokens {
			if err := jwtProvider.ValidateAccessToken(mt); err == nil {
				t.Errorf("Expected error for malformed token '%s'", mt)
			}
			if err := jwtProvider.ValidateRefreshToken(mt); err == nil {
				t.Errorf("Expected error for malformed token '%s'", mt)
			}
			if _, err := jwtProvider.GetUUIDFromToken(mt); err == nil {
				t.Errorf("Expected error extracting UUID from malformed token '%s'", mt)
			}
		}
	})
}

func TestAuthService_TokenRefresh(t *testing.T) {
	ctx := context.Background()
	userSvc := &MockUserSvc{users: make(map[string]User)}
	jwtProvider := setupTestJwtProvider(t)
	authSvc := &AuthService{UserSvc: userSvc, JwtProvider: jwtProvider}

	user := User{
		UUID:         "user-uuid-1234",
		Login:        "refreshuser",
		PasswordHash: "somehash",
	}
	_ = userSvc.SaveUser(ctx, user)

	refreshToken, err := jwtProvider.GenerateRefreshToken(&user)
	if err != nil {
		t.Fatalf("Failed to generate refresh token: %v", err)
	}

	t.Run("UpdateAccessToken success", func(t *testing.T) {
		resp, err := authSvc.UpdateAccessToken(ctx, refreshToken)
		if err != nil {
			t.Fatalf("UpdateAccessToken failed: %v", err)
		}
		if resp == nil {
			t.Fatalf("Expected non-nil JwtResponse")
		}
		if resp.Type != "Bearer" {
			t.Errorf("Expected Type 'Bearer', got '%s'", resp.Type)
		}
		if resp.RefreshToken != refreshToken {
			t.Errorf("Expected RefreshToken to be preserved, got '%s'", resp.RefreshToken)
		}
		if err := jwtProvider.ValidateAccessToken(resp.AccessToken); err != nil {
			t.Errorf("Generated AccessToken is invalid: %v", err)
		}
		uuid, _ := jwtProvider.GetUUIDFromToken(resp.AccessToken)
		if uuid != user.UUID {
			t.Errorf("Expected UUID %s in access token, got %s", user.UUID, uuid)
		}
	})

	t.Run("UpdateRefreshToken success", func(t *testing.T) {
		resp, err := authSvc.UpdateRefreshToken(ctx, refreshToken)
		if err != nil {
			t.Fatalf("UpdateRefreshToken failed: %v", err)
		}
		if resp == nil {
			t.Fatalf("Expected non-nil JwtResponse")
		}
		if resp.Type != "Bearer" {
			t.Errorf("Expected Type 'Bearer', got '%s'", resp.Type)
		}
		if err := jwtProvider.ValidateAccessToken(resp.AccessToken); err != nil {
			t.Errorf("Generated AccessToken is invalid: %v", err)
		}
		if err := jwtProvider.ValidateRefreshToken(resp.RefreshToken); err != nil {
			t.Errorf("Generated RefreshToken is invalid: %v", err)
		}
		uuid, _ := jwtProvider.GetUUIDFromToken(resp.RefreshToken)
		if uuid != user.UUID {
			t.Errorf("Expected UUID %s in refresh token, got %s", user.UUID, uuid)
		}
	})

	t.Run("Refresh with invalid or wrong token type", func(t *testing.T) {
		accessToken, _ := jwtProvider.GenerateAccessToken(&user)

		// Passing access token to UpdateAccessToken
		if _, err := authSvc.UpdateAccessToken(ctx, accessToken); err == nil {
			t.Errorf("Expected error when passing access token to UpdateAccessToken")
		}

		// Passing access token to UpdateRefreshToken
		if _, err := authSvc.UpdateRefreshToken(ctx, accessToken); err == nil {
			t.Errorf("Expected error when passing access token to UpdateRefreshToken")
		}

		// Tampered token
		if _, err := authSvc.UpdateAccessToken(ctx, refreshToken+"tampered"); err == nil {
			t.Errorf("Expected error when passing tampered token to UpdateAccessToken")
		}
		if _, err := authSvc.UpdateRefreshToken(ctx, refreshToken+"tampered"); err == nil {
			t.Errorf("Expected error when passing tampered token to UpdateRefreshToken")
		}
	})

	t.Run("Refresh for deleted or non-existent user", func(t *testing.T) {
		ghostUser := &User{
			UUID:  "ghost-uuid",
			Login: "ghost",
		}
		ghostRefreshToken, _ := jwtProvider.GenerateRefreshToken(ghostUser)

		_, err := authSvc.UpdateAccessToken(ctx, ghostRefreshToken)
		var incCredsErr *IncorrectCredsError
		if !errors.As(err, &incCredsErr) {
			t.Errorf("Expected IncorrectCredsError for non-existent user in UpdateAccessToken, got %v", err)
		}

		_, err = authSvc.UpdateRefreshToken(ctx, ghostRefreshToken)
		if !errors.As(err, &incCredsErr) {
			t.Errorf("Expected IncorrectCredsError for non-existent user in UpdateRefreshToken, got %v", err)
		}
	})
}
