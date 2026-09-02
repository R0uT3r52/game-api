package domain

import (
	"context"
	"errors"
	"testing"
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

func TestAuthService_RegisterAndAuthorize(t *testing.T) {
	ctx := context.Background()
	userSvc := &MockUserSvc{users: make(map[string]User)}
	authSvc := &AuthService{UserSvc: userSvc}

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
	uuid, err := authSvc.Authorize(ctx, "alice", "alicepassword")
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}
	if uuid == "" {
		t.Fatalf("Expected non-empty UUID")
	}

	// 4. Wrong password
	_, err = authSvc.Authorize(ctx, "alice", "wrongpassword")
	var incorrectCredsErr *IncorrectCredsError
	if !errors.As(err, &incorrectCredsErr) {
		t.Errorf("Expected IncorrectCredsError, got %v", err)
	}

	// 5. Non-existent user
	_, err = authSvc.Authorize(ctx, "bob", "bobpassword")
	if !errors.As(err, &incorrectCredsErr) {
		t.Errorf("Expected IncorrectCredsError for non-existent user, got %v", err)
	}

	// 8. GetUser
	user, err := authSvc.GetUser(ctx, uuid)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if user == nil || user.Login != "alice" {
		t.Errorf("GetUser returned wrong user: %v", user)
	}

	// 9. Empty Register
	req2 := SignUpRequest{
		Login:    "",
		Password: "",
	}

	err2 := authSvc.Register(ctx, req2)
	if err2 == nil {
		t.Errorf("Registered empty user. login: '%s', password: '%s'", req2.Login, req2.Password)
	}
}
