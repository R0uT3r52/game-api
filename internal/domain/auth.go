package domain

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Register(req SignUpRequest) error
// Authorize(authHeader string) (uuid string, err error)

func parseHeader(header string) (login, password string, err error) {
	data := header

	if len(header) >= 6 && strings.EqualFold(header[:6], "basic ") {
		data = header[6:]
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", "", fmt.Errorf("auth header decode error: %w", err)
	}

	creds := strings.SplitN(string(decodedBytes), ":", 2)
	if len(creds) != 2 {
		return "", "", errors.New("auth header: invalid credentials format")
	}

	return creds[0], creds[1], nil
}

func (au *AuthService) Register(req SignUpRequest) error {
	existingUser, err := au.UserSvc.GetUserByLogin(req.Login)
	if err != nil {
		return err
	}
	if existingUser != nil {
		return &UserAlreadyExistsError{Login: req.Login}
	}

	id := uuid.New()

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	err = au.UserSvc.SaveUser(User{
		UUID:         id.String(),
		Login:        req.Login,
		PasswordHash: string(hashedPwd),
	})
	return err
}

func (au *AuthService) Authorize(authHeader string) (uuid string, err error) {
	login, password, err := parseHeader(authHeader)
	if err != nil {
		return "", err
	}

	user, err := au.UserSvc.GetUserByLogin(login)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", &IncorrectCredsError{
			login: login,
			pass:  password,
		}
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", &IncorrectCredsError{
			login: login,
			pass:  password,
		}
	}

	return user.UUID, nil
}
