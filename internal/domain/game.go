package domain

import (
	"context"

	"github.com/google/uuid"
)

// MakeAiMove(uuid string) (*Session, error)
// ValidateField(uuid string, newField Field) error
// CheckGameEnd(uuid string) (isEnded bool, winner int, err error)

func NewGameService(repo GameRepositoryInterface, pSign, bSign int) *GameService {
	return &GameService{
		repo:       repo,
		PlayerSign: pSign,
		BotSign:    bSign,
	}
}

// Returns uuid of game
//
// Error if unable to save game
func (g *GameService) CreateGame(ctx context.Context, p1 string, withBot bool) (id string, err error) {

	uid := uuid.New()

	status := Waiting
	player2Sign := Empty
	if withBot {
		status = Turn
		player2Sign = Nought
	}

	err = g.repo.Save(ctx, &Session{
		UUID:            uid.String(),
		F:               Field{},
		Player1UUID:     p1,
		Player2UUID:     "",
		CurrentTurnUUID: p1,

		Status:      status,
		IsWithBot:   withBot,
		WinnerUUID:  "",
		Player1Sign: Cross,
		Player2Sign: player2Sign,
	})

	return uid.String(), err
}

func (g *GameService) MakeMove(ctx context.Context, gameUUID, playerUUID string, newField Field) (*Session, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	s, err := g.repo.Load(ctx, gameUUID)
	if err != nil {
		return nil, err
	}

	if s.Status != Turn {
		return nil, &ValidationError{Message: "game is not in turn state"}
	}

	if s.CurrentTurnUUID != playerUUID {
		return nil, &ValidationError{Message: "it is not your turn"}
	}

	var playerSign int
	if playerUUID == s.Player1UUID {
		playerSign = s.Player1Sign
	} else if playerUUID == s.Player2UUID {
		playerSign = s.Player2Sign
	} else {
		return nil, &ValidationError{Message: "you are not a player in this game"}
	}

	if err := g.validateGridChange(s.F, newField, playerSign); err != nil {
		return nil, err
	}

	s.F = newField

	err = g.repo.Save(ctx, s)
	if err != nil {
		return nil, err
	}

	// Check for end after move
	ended, winner, _ := g.CheckGameEnd(ctx, s.UUID)
	if ended {
		g.applyEndState(s, winner)
		err = g.repo.Save(ctx, s)
		return s, err
	}

	// Switch turn
	if !s.IsWithBot {
		if s.CurrentTurnUUID == s.Player1UUID {
			s.CurrentTurnUUID = s.Player2UUID
		} else {
			s.CurrentTurnUUID = s.Player1UUID
		}
	} else {
		// If with bot, player move is done, save it, then call AI move
		err = g.repo.Save(ctx, s)
		if err != nil {
			return nil, err
		}
		return g.MakeAiMove(ctx, s.UUID)
	}

	err = g.repo.Save(ctx, s)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (g *GameService) validateGridChange(oldField, newField Field, expectedSign int) error {
	changes := 0
	for i := range newField.Grid {
		for j := range newField.Grid {
			if oldField.Grid[i][j] != newField.Grid[i][j] {
				changes++
				if oldField.Grid[i][j] != Empty {
					return &ValidationError{Message: "player cannot overwrite cell"}
				}
				if newField.Grid[i][j] != expectedSign {
					return &ValidationError{Message: "incorrect player sign"}
				}
			}
		}
	}

	if changes != 1 {
		return &ValidationError{Message: "player must change exactly one cell"}
	}
	return nil
}

func (g *GameService) applyEndState(s *Session, winner int) {
	if winner == Tie {
		s.Status = Draw
		s.CurrentTurnUUID = ""
	} else if winner == Player {
		s.Status = Win
		s.WinnerUUID = s.Player1UUID
		s.CurrentTurnUUID = ""
	} else if winner == Bot {
		s.Status = Win
		if s.IsWithBot {
			// "bot" will cause DB error because of types in PostgreSQL
			// s.WinnerUUID = "bot"
			s.WinnerUUID = ""
		} else {
			s.WinnerUUID = s.Player2UUID
		}
		s.CurrentTurnUUID = ""
	}
}

func (g *GameService) GetAvailableGames(ctx context.Context) ([]Session, error) {
	ans, err := g.repo.ListAvailable(ctx)
	if err != nil {
		return nil, err
	}

	return ans, nil
}

func (g *GameService) GetCurrentGames(ctx context.Context, gameUUID, playerUUID string) ([]Session, error) {
	ans, err := g.repo.GetCurrentGames(ctx, gameUUID, playerUUID)

	if err != nil {
		return nil, err
	}

	return ans, nil
}

func (g *GameService) Connect(ctx context.Context, p2UUID, gameUUID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	s, err := g.repo.Load(ctx, gameUUID)
	if err != nil {
		return err
	}

	if s.Status != Waiting {
		return ErrGameAlreadyStarted
	}

	if p2UUID == s.Player1UUID {
		return ErrUserAlreadyInGame
	}

	s.Status = Turn
	s.CurrentTurnUUID = s.Player1UUID
	s.Player2Sign = Nought
	s.Player2UUID = p2UUID

	err = g.repo.Save(ctx, s)
	if err != nil {
		return err
	}

	return nil
}

func (g *GameService) ValidateField(ctx context.Context, uuid string, newField Field) error {
	s, err := g.repo.Load(ctx, uuid)

	if err != nil || s == nil {
		s = &Session{UUID: uuid}
		err = g.repo.Save(ctx, s)
		if err != nil {
			return err
		}
	} else {
		// check if game finished (no more moves allowed)
		if g.evaluate(&s.F) != 0 || !isMovesLeft(&s.F) {
			return &ValidationError{Message: "game is already finished"}
		}
	}

	if err := g.validateGridChange(s.F, newField, g.PlayerSign); err != nil {
		return err
	}

	s.F = newField
	return g.repo.Save(ctx, s)
}

func (g *GameService) MakeAiMove(ctx context.Context, uuid string) (*Session, error) {
	s, err := g.repo.Load(ctx, uuid)
	if err != nil {
		return nil, &GameNotFoundError{UUID: uuid}
	}

	r, c := g.getNextMove(&s.F)
	if r == -1 || c == -1 {
		return nil, &ValidationError{Message: "error"}
	}

	s.F.Grid[r][c] = g.BotSign

	err = g.repo.Save(ctx, s)
	if err != nil {
		return s, err
	}

	// Check for end after AI move
	ended, winner, _ := g.CheckGameEnd(ctx, s.UUID)
	if ended {
		g.applyEndState(s, winner)
		err = g.repo.Save(ctx, s)
		if err != nil {
			return s, err
		}
	}

	return s, nil
}

func (g *GameService) CheckGameEnd(ctx context.Context, uuid string) (isEnded bool, winner int, err error) {
	s, err := g.repo.Load(ctx, uuid)
	if err != nil {
		return false, -1, &GameNotFoundError{UUID: uuid}
	}

	score := g.evaluate(&s.F)

	if score == 10 {
		return true, Bot, nil
	} else if score == -10 {
		return true, Player, nil
	} else if score == 0 && !isMovesLeft(&s.F) {
		return true, Tie, nil
	}

	return false, -1, nil
}

func (g *GameService) getNextMove(f *Field) (row, col int) {
	best := -1000
	r := -1
	c := -1

	for i := range f.Grid {
		for j := range f.Grid {
			if f.Grid[i][j] == Empty {
				f.Grid[i][j] = g.BotSign

				moveVal := g.miniMax(f, 0, false)

				f.Grid[i][j] = Empty

				if moveVal > best {
					r = i
					c = j
					best = moveVal
				}

			}
		}
	}

	return r, c
}

func (g *GameService) evaluate(f *Field) int {
	for row := range f.Grid {
		if f.Grid[row][0] == f.Grid[row][1] && f.Grid[row][1] == f.Grid[row][2] {
			if f.Grid[row][0] == g.PlayerSign {
				return -10
			} else if f.Grid[row][0] == g.BotSign {
				return 10
			}
		}
	}

	for col := range f.Grid {
		if f.Grid[0][col] == f.Grid[1][col] && f.Grid[1][col] == f.Grid[2][col] {
			if f.Grid[0][col] == g.PlayerSign {
				return -10
			} else if f.Grid[0][col] == g.BotSign {
				return 10
			}
		}
	}

	if f.Grid[0][0] == f.Grid[1][1] && f.Grid[1][1] == f.Grid[2][2] {
		if f.Grid[0][0] == g.PlayerSign {
			return -10
		} else if f.Grid[0][0] == g.BotSign {
			return 10
		}
	}

	if f.Grid[0][2] == f.Grid[1][1] && f.Grid[1][1] == f.Grid[2][0] {
		if f.Grid[0][2] == g.PlayerSign {
			return -10
		} else if f.Grid[0][2] == g.BotSign {
			return 10
		}
	}

	return 0
}

func isMovesLeft(f *Field) bool {
	for i := range f.Grid {
		for j := range f.Grid {
			if f.Grid[i][j] == Empty {
				return true
			}
		}
	}
	return false
}

func (g *GameService) miniMax(f *Field, depth int, isMax bool) int {
	score := g.evaluate(f)
	if score == 10 {
		return score - depth
	}
	if score == -10 {
		return score + depth
	}

	if !isMovesLeft(f) {
		return 0
	}

	if isMax {
		best := -1000
		for i := range f.Grid {
			for j := range f.Grid {
				if f.Grid[i][j] == Empty {
					f.Grid[i][j] = g.BotSign

					best = max(best, g.miniMax(f, depth+1, !isMax))

					f.Grid[i][j] = Empty
				}
			}
		}

		return best
	} else {
		best := 1000
		for i := range f.Grid {
			for j := range f.Grid {
				if f.Grid[i][j] == Empty {
					f.Grid[i][j] = g.PlayerSign

					best = min(best, g.miniMax(f, depth+1, !isMax))

					f.Grid[i][j] = Empty
				}
			}
		}

		return best
	}

}
