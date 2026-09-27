package domain

import "fmt"

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s", e.Message)
}

type GameNotFoundError struct {
	UUID string
}

func (e *GameNotFoundError) Error() string {
	return fmt.Sprintf("game not found: %s", e.UUID)
}

type IncorrectCredsError struct {
	login string
}

func (e *IncorrectCredsError) Error() string {
	return fmt.Sprintf("incorrect credentials error. Login: %s", e.login)
}

type UserAlreadyExistsError struct {
	Login string
}

func (e *UserAlreadyExistsError) Error() string {
	return fmt.Sprintf("user already exists error. Login: %s", e.Login)
}

type InvalidTokenError struct {
	Token string
}

func (e *InvalidTokenError) Error() string {
	return fmt.Sprintf("invalid token: %s", e.Token)
}

type JwtSecretError struct{}

func (e *JwtSecretError) Error() string {
	return fmt.Sprintf("unable to find JWT_SECRET env variable")
}

var ErrGameAlreadyStarted = &ValidationError{Message: "Game already started with another player"}
var ErrUserAlreadyInGame = &ValidationError{Message: "User already in this game"}
