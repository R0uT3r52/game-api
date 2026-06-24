package domain

import "fmt"

// MakeAiMove(uuid string) (*Session, error)
// ValidateField(uuid string, newField Field) error
// CheckGameEnd(uuid string) (isEnded bool, winner int, err error)

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s\n", e.Message)
}

func (e *GameNotFoundError) Error() string {
	return fmt.Sprintf("game not found: %s\n", e.UUID)
}

func NewGameService(repo GameRepositoryInterface, pSign, bSign int) *GameService {
	return &GameService{
		repo:       repo,
		PlayerSign: pSign,
		BotSign:    bSign,
	}
}

func (g *GameService) ValidateField(uuid string, newField Field) error {
	s, err := g.repo.Load(uuid)

	if err != nil {
		s = &Session{UUID: uuid}
		g.repo.Save(s)
	}

	changes := 0

	for i := range newField.Grid {
		for j := range newField.Grid {
			if s.F.Grid[i][j] != newField.Grid[i][j] {
				changes++
				if s.F.Grid[i][j] != Empty {
					return &ValidationError{Message: "player cannot overwrite cell"}
				}
				if newField.Grid[i][j] != g.PlayerSign {
					return &ValidationError{Message: "incorrect player sign"}
				}
			}
		}
	}

	if changes != 1 {
		return &ValidationError{Message: "player must change only one cell"}
	}

	s.F = newField

	err = g.repo.Save(s)

	if err != nil {
		return err
	}

	return nil
}

func (g *GameService) MakeAiMove(uuid string) (*Session, error) {
	s, err := g.repo.Load(uuid)
	if err != nil {
		return nil, &GameNotFoundError{UUID: uuid}
	}

	r, c := g.getNextMove(&s.F)
	if r == -1 || c == -1 {
		return nil, &ValidationError{Message: "error"}
	}

	s.F.Grid[r][c] = g.BotSign

	err = g.repo.Save(s)
	if err != nil {
		// Basically move made, but unable to save
		return s, err
	}
	// Move made and was able to save
	return s, nil
}

func (g *GameService) CheckGameEnd(uuid string) (isEnded bool, winner int, err error) {
	s, err := g.repo.Load(uuid)
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
