package datasource

import "fmt"

type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("not found error: %s\n", e.Message)
}

type DBConnectionError struct {
	Message string
}

func (e *DBConnectionError) Error() string {
	return fmt.Sprintf("DB connection error: %s\n", e.Message)
}
