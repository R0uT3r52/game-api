package domain

import (
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

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

func (au *AuthService) Authorize(login, password string) (uuid string, err error) {
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

func (au *AuthService) GetUser(uuid string) (*User, error) {
	return au.UserSvc.GetUser(uuid)
}
