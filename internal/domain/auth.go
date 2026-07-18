package domain

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type IncorrectCredsError struct {
	login string
	pass  string
}

func (e *IncorrectCredsError) Error() string {
	return fmt.Sprintf("incorrect credentials error. Login: %s, Password: %s", e.login, e.pass)
}

// Register(req SignUpRequest) error
// Authorize(authHeader string) (uuid string, err error)

func parseHeader(header string) (login, password string, err error) {
	const prefix = "Basic "

	if !strings.HasPrefix(header, prefix) {
		return "", "", errors.New("invalid auth header format")
	}

	data := header[len(prefix):]

	decodedBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", "", err
	}

	creds := strings.SplitN(string(decodedBytes), ":", 2)
	if len(creds) != 2 {
		return "", "", errors.New("invalid credentials format")
	}

	return creds[0], creds[1], nil
}

func (au *AuthService) Register(req SignUpRequest) error {
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

	user, err := au.UserSvc.GetUserByLogin(login)
	if err != nil {
		return "", err
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
