package domain

import (
	"encoding/base64"
	"errors"
	"testing"
)

type MockUserSvc struct {
	users map[string]User
}

func (m *MockUserSvc) GetUser(uuid string) (*User, error) {
	u, ok := m.users[uuid]
	if !ok {
		return nil, nil
	}
	return &u, nil
}

func (m *MockUserSvc) GetUserByLogin(login string) (*User, error) {
	for _, u := range m.users {
		if u.Login == login {
			return &u, nil
		}
	}
	return nil, nil
}

func (m *MockUserSvc) SaveUser(u User) error {
	if m.users == nil {
		m.users = make(map[string]User)
	}
	m.users[u.UUID] = u
	return nil
}

func TestAuthService_RegisterAndAuthorize(t *testing.T) {
	userSvc := &MockUserSvc{users: make(map[string]User)}
	authSvc := &AuthService{UserSvc: userSvc}

	// 1. Success Register
	req := SignUpRequest{
		Login:    "alice",
		Password: "alicepassword",
	}

	err := authSvc.Register(req)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 2. Duplicate Register
	err = authSvc.Register(req)
	var duplicateErr *UserAlreadyExistsError
	if !errors.As(err, &duplicateErr) {
		t.Errorf("Expected UserAlreadyExistsError, got %v", err)
	}

	// 3. Success Authorize
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:alicepassword"))
	uuid, err := authSvc.Authorize(authHeader)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}
	if uuid == "" {
		t.Fatalf("Expected non-empty UUID")
	}

	// 4. Invalid Header format (no space after Basic)
	_, err = authSvc.Authorize("Basic" + base64.StdEncoding.EncodeToString([]byte("alice:alicepassword")))
	if err == nil {
		t.Errorf("Expected authorization to fail due to missing space in header")
	}

	// 5. Invalid credentials format (no colon)
	badCredsHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("alicepassword"))
	_, err = authSvc.Authorize(badCredsHeader)
	if err == nil {
		t.Errorf("Expected authorization to fail due to invalid credentials format")
	}

	// 6. Wrong password
	wrongPassHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:wrongpassword"))
	_, err = authSvc.Authorize(wrongPassHeader)
	var incorrectCredsErr *IncorrectCredsError
	if !errors.As(err, &incorrectCredsErr) {
		t.Errorf("Expected IncorrectCredsError, got %v", err)
	}

	// 7. Non-existent user
	nonExistentHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte("bob:bobpassword"))
	_, err = authSvc.Authorize(nonExistentHeader)
	if !errors.As(err, &incorrectCredsErr) {
		t.Errorf("Expected IncorrectCredsError for non-existent user, got %v", err)
	}
}
