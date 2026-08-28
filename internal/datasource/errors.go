package datasource

import "fmt"

type DBConnectionError struct {
	Message string
}

func (e *DBConnectionError) Error() string {
	return fmt.Sprintf("DB connection error: %s", e.Message)
}
