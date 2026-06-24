package domain

// Cell states
const (
	Empty = iota
	Cross
	Nought
)

// Win states
const (
	Player = iota
	Bot
	Tie
)

type Field struct {
	Grid [3][3]int
}

type Session struct {
	UUID string
	F    Field
}

type GameService struct {
	repo       GameRepositoryInterface
	PlayerSign int
	BotSign    int
}

type ValidationError struct {
	Message string
}

type GameNotFoundError struct {
	UUID string
}
