package domain

// Cell states
const (
	Empty = iota
	Cross
	Nought
)

const (
	Player = iota
	Bot
	Tie
)

// TODO:
// Fix global variables later
// var PlayerSign int = Cross
// var BotSign int = Nought

// ???
type Field struct {
	Grid [3][3]int
}

// ???
type Session struct {
	UUID string
	F    Field
}

// ???
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
