package domain

import "fmt"

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s\n", e.Message)
}

type GameNotFoundError struct {
	UUID string
}

func (e *GameNotFoundError) Error() string {
	return fmt.Sprintf("game not found: %s\n", e.UUID)
}

type IncorrectCredsError struct {
	login string
	pass  string
}

func (e *IncorrectCredsError) Error() string {
	return fmt.Sprintf("incorrect credentials error. Login: %s, Password: %s", e.login, e.pass)
}

type UserAlreadyExistsError struct {
	Login string
}

func (e *UserAlreadyExistsError) Error() string {
	return fmt.Sprintf("user already exists error. Login: %s", e.Login)
}

var ErrGameAlreadyStarted = &ValidationError{Message: "Game already started with another player"}

