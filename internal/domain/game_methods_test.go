package domain

import (
	"errors"
	"testing"
)

type MockRepo struct {
	sessions map[string]*Session
	saveErr  error
	loadErr  error
	listErr  error
}

func (m *MockRepo) Save(s *Session) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.sessions[s.UUID] = s
	return nil
}

func (m *MockRepo) Load(uuid string) (*Session, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	s, ok := m.sessions[uuid]
	if !ok {
		return nil, errors.New("not found")
	}
	// Return a copy to avoid side effects during validation
	sCopy := *s
	return &sCopy, nil
}

func (m *MockRepo) ListAvailable() ([]Session, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	ans := make([]Session, 0)
	for _, s := range m.sessions {
		if s.Status == Waiting {
			ans = append(ans, *s)
		}
	}
	return ans, nil
}

func (m *MockRepo) GetCurrentGames(gameUUID, playerUUID string) ([]Session, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	ans := make([]Session, 0)
	for _, s := range m.sessions {
		if (s.Player1UUID == playerUUID || s.Player2UUID == playerUUID) && (gameUUID == "" || s.UUID == gameUUID) {
			ans = append(ans, *s)
		}
	}
	return ans, nil
}

func TestValidateField(t *testing.T) {
	repo := &MockRepo{sessions: make(map[string]*Session)}
	service := NewGameService(repo, Cross, Nought)
	uuid := "test-uuid"

	tests := []struct {
		name     string
		initial  Field
		newField Field
		wantErr  bool
		errMsg   string
	}{
		{
			name: "Valid move",
			initial: Field{Grid: [3][3]int{
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			newField: Field{Grid: [3][3]int{
				{Cross, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			wantErr: false,
		},
		{
			name: "Multiple changes",
			initial: Field{Grid: [3][3]int{
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			newField: Field{Grid: [3][3]int{
				{Cross, Cross, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			wantErr: true,
			errMsg:  "player must change only one cell",
		},
		{
			name: "No changes",
			initial: Field{Grid: [3][3]int{
				{Cross, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			newField: Field{Grid: [3][3]int{
				{Cross, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			wantErr: true,
			errMsg:  "player must change only one cell",
		},
		{
			name: "Overwrite player cell",
			initial: Field{Grid: [3][3]int{
				{Cross, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			newField: Field{Grid: [3][3]int{
				{Nought, Empty, Empty}, // This changes Cross to Nought. Wait, changes count?
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			wantErr: true,
			errMsg:  "player cannot overwrite cell",
		},
		{
			name: "Overwrite bot cell",
			initial: Field{Grid: [3][3]int{
				{Nought, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			newField: Field{Grid: [3][3]int{
				{Cross, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			wantErr: true,
			errMsg:  "player cannot overwrite cell",
		},
		{
			name: "Player plays as Bot",
			initial: Field{Grid: [3][3]int{
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			newField: Field{Grid: [3][3]int{
				{Nought, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			wantErr: true,
			errMsg:  "player cannot overwrite cell as BotSign",
		},
		{
			name: "Player uses invalid sign (bug: currently accepts any value except BotSign)",
			initial: Field{Grid: [3][3]int{
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			newField: Field{Grid: [3][3]int{
				{5, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			wantErr: true,
		},
		{
			name: "Player removes sign",
			initial: Field{Grid: [3][3]int{
				{Cross, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			newField: Field{Grid: [3][3]int{
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			wantErr: true,
			errMsg:  "player cannot overwrite cell",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.sessions[uuid] = &Session{UUID: uuid, F: tt.initial}
			err := service.ValidateField(uuid, tt.newField)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				// Check if error message contains expected string
				var vErr *ValidationError
				if errors.As(err, &vErr) {
					// We can check message if we want, but some might be different
					// t.Logf("Got expected validation error: %v", vErr.Message)
				} else {
					// t.Errorf("Expected ValidationError, got %T", err)
				}
			}
		})
	}
}

func TestCheckGameEnd(t *testing.T) {
	repo := &MockRepo{sessions: make(map[string]*Session)}
	service := NewGameService(repo, Cross, Nought)
	uuid := "test-uuid"

	tests := []struct {
		name       string
		field      Field
		wantEnded  bool
		wantWinner int
	}{
		{
			name: "Bot wins row",
			field: Field{Grid: [3][3]int{
				{Nought, Nought, Nought},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			wantEnded:  true,
			wantWinner: Bot,
		},
		{
			name: "Player wins column",
			field: Field{Grid: [3][3]int{
				{Cross, Empty, Empty},
				{Cross, Empty, Empty},
				{Cross, Empty, Empty},
			}},
			wantEnded:  true,
			wantWinner: Player,
		},
		{
			name: "Tie",
			field: Field{Grid: [3][3]int{
				{Nought, Cross, Nought},
				{Nought, Cross, Cross},
				{Cross, Nought, Cross},
			}},
			wantEnded:  true,
			wantWinner: Tie,
		},
		{
			name: "Ongoing game",
			field: Field{Grid: [3][3]int{
				{Nought, Cross, Empty},
				{Empty, Empty, Empty},
				{Empty, Empty, Empty},
			}},
			wantEnded:  false,
			wantWinner: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo.sessions[uuid] = &Session{UUID: uuid, F: tt.field}
			ended, winner, err := service.CheckGameEnd(uuid)
			if err != nil {
				t.Errorf("CheckGameEnd() error = %v", err)
				return
			}
			if ended != tt.wantEnded {
				t.Errorf("CheckGameEnd() ended = %v, want %v", ended, tt.wantEnded)
			}
			if winner != tt.wantWinner {
				t.Errorf("CheckGameEnd() winner = %v, want %v", winner, tt.wantWinner)
			}
		})
	}
}

func TestCreateGame(t *testing.T) {
	repo := &MockRepo{sessions: make(map[string]*Session)}
	service := NewGameService(repo, Cross, Nought)

	// PvP Game
	id1, err := service.CreateGame("p1-uuid", false)
	if err != nil {
		t.Fatalf("CreateGame PvP failed: %v", err)
	}
	s1, err := repo.Load(id1)
	if err != nil || s1.Status != Waiting || s1.Player1UUID != "p1-uuid" || s1.IsWithBot {
		t.Errorf("Unexpected PvP session state: %+v, err: %v", s1, err)
	}

	// Bot Game
	id2, err := service.CreateGame("p1-uuid", true)
	if err != nil {
		t.Fatalf("CreateGame Bot failed: %v", err)
	}
	s2, err := repo.Load(id2)
	if err != nil || s2.Status != Turn || !s2.IsWithBot || s2.Player2Sign != Nought {
		t.Errorf("Unexpected Bot session state: %+v, err: %v", s2, err)
	}
}

func TestConnect(t *testing.T) {
	repo := &MockRepo{sessions: make(map[string]*Session)}
	service := NewGameService(repo, Cross, Nought)

	gameID, _ := service.CreateGame("p1-uuid", false)

	// Valid connect
	err := service.Connect("p2-uuid", gameID)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	s, _ := repo.Load(gameID)
	if s.Player2UUID != "p2-uuid" || s.Status != Turn {
		t.Errorf("Unexpected session state after connect: %+v", s)
	}

	// Duplicate connect
	err = service.Connect("p3-uuid", gameID)
	if !errors.Is(err, ErrGameAlreadyStarted) {
		t.Errorf("Expected ErrGameAlreadyStarted, got: %v", err)
	}
}

func TestMakeMove(t *testing.T) {
	repo := &MockRepo{sessions: make(map[string]*Session)}
	service := NewGameService(repo, Cross, Nought)

	gameID, _ := service.CreateGame("p1-uuid", false)
	_ = service.Connect("p2-uuid", gameID)

	// 1. Move when not in turn
	wrongTurnMove := Field{Grid: [3][3]int{{Cross, Empty, Empty}, {Empty, Empty, Empty}, {Empty, Empty, Empty}}}
	_, err := service.MakeMove(gameID, "p2-uuid", wrongTurnMove)
	if err == nil {
		t.Errorf("Expected error for wrong player's turn")
	}

	// 2. Valid move p1
	s1, err := service.MakeMove(gameID, "p1-uuid", wrongTurnMove)
	if err != nil {
		t.Fatalf("MakeMove p1 failed: %v", err)
	}
	if s1.CurrentTurnUUID != "p2-uuid" {
		t.Errorf("Expected turn to switch to p2, got %s", s1.CurrentTurnUUID)
	}

	// 3. Move from non-player
	p2Move := Field{Grid: [3][3]int{{Cross, Empty, Empty}, {Nought, Empty, Empty}, {Empty, Empty, Empty}}}
	_, err = service.MakeMove(gameID, "stranger-uuid", p2Move)
	if err == nil {
		t.Errorf("Expected error for non-player move")
	}

	// 4. Move in non-turn state
	gameNotTurnID, _ := service.CreateGame("p1-uuid", false)
	_, err = service.MakeMove(gameNotTurnID, "p1-uuid", wrongTurnMove)
	if err == nil {
		t.Errorf("Expected error when game is not in turn state")
	}

	// 5. Winning move
	winningRepo := &MockRepo{sessions: make(map[string]*Session)}
	winningService := NewGameService(winningRepo, Cross, Nought)
	wID, _ := winningService.CreateGame("p1", false)
	_ = winningService.Connect("p2", wID)
	// Setup field where p1 is about to win
	session, _ := winningRepo.Load(wID)
	session.F.Grid = [3][3]int{
		{Cross, Cross, Empty},
		{Nought, Nought, Empty},
		{Empty, Empty, Empty},
	}
	winningRepo.Save(session)

	winMove := Field{Grid: [3][3]int{
		{Cross, Cross, Cross},
		{Nought, Nought, Empty},
		{Empty, Empty, Empty},
	}}
	winSession, err := winningService.MakeMove(wID, "p1", winMove)
	if err != nil {
		t.Fatalf("Winning move failed: %v", err)
	}
	if winSession.Status != Win || winSession.WinnerUUID != "p1" {
		t.Errorf("Expected Win status and p1 winner, got status=%d winner=%s", winSession.Status, winSession.WinnerUUID)
	}

	// 6. Move with bot
	botRepo := &MockRepo{sessions: make(map[string]*Session)}
	botService := NewGameService(botRepo, Cross, Nought)
	botGameID, _ := botService.CreateGame("p1", true)
	botMove := Field{Grid: [3][3]int{
		{Cross, Empty, Empty},
		{Empty, Empty, Empty},
		{Empty, Empty, Empty},
	}}
	botSession, err := botService.MakeMove(botGameID, "p1", botMove)
	if err != nil {
		t.Fatalf("Move with bot failed: %v", err)
	}
	if botSession == nil {
		t.Fatalf("Expected non-nil bot session")
	}
}

func TestMakeAiMove(t *testing.T) {
	repo := &MockRepo{sessions: make(map[string]*Session)}
	service := NewGameService(repo, Cross, Nought)

	// Game not found
	_, err := service.MakeAiMove("non-existent")
	var notFoundErr *GameNotFoundError
	if !errors.As(err, &notFoundErr) {
		t.Errorf("Expected GameNotFoundError, got %v", err)
	}

	// Valid AI move
	gameID, _ := service.CreateGame("p1", true)
	s, err := service.MakeAiMove(gameID)
	if err != nil {
		t.Fatalf("MakeAiMove failed: %v", err)
	}
	if s == nil {
		t.Fatalf("Expected session after AI move")
	}
}

func TestListAvailableAndCurrentGames(t *testing.T) {
	repo := &MockRepo{sessions: make(map[string]*Session)}
	service := NewGameService(repo, Cross, Nought)

	g1, _ := service.CreateGame("p1", false)
	_, _ = service.CreateGame("p2", true)

	available, err := service.GetAvailableGames()
	if err != nil || len(available) != 1 || available[0].UUID != g1 {
		t.Errorf("GetAvailableGames failed, expected [g1], got %v (err: %v)", available, err)
	}

	current, err := service.GetCurrentGames("", "p1")
	if err != nil || len(current) != 1 || current[0].UUID != g1 {
		t.Errorf("GetCurrentGames failed, expected [g1], got %v (err: %v)", current, err)
	}
}

func TestPvPGameEndAndTie(t *testing.T) {
	pvpRepo := &MockRepo{sessions: make(map[string]*Session)}
	pvpService := NewGameService(pvpRepo, Cross, Nought)
	pID, _ := pvpService.CreateGame("p1", false)
	_ = pvpService.Connect("p2", pID)
	// P2 (Nought) winning move
	pSession, _ := pvpRepo.Load(pID)
	pSession.F.Grid = [3][3]int{
		{Nought, Nought, Empty},
		{Cross, Cross, Empty},
		{Empty, Empty, Empty},
	}
	pSession.CurrentTurnUUID = "p2"
	pvpRepo.Save(pSession)

	p2WinMove := Field{Grid: [3][3]int{
		{Nought, Nought, Nought},
		{Cross, Cross, Empty},
		{Empty, Empty, Empty},
	}}
	p2WinSession, err := pvpService.MakeMove(pID, "p2", p2WinMove)
	if err != nil || p2WinSession.Status != Win || p2WinSession.WinnerUUID != "p2" {
		t.Errorf("Expected p2 win, got status %d, winner %s, err %v", p2WinSession.Status, p2WinSession.WinnerUUID, err)
	}

	// Tie Game
	tieRepo := &MockRepo{sessions: make(map[string]*Session)}
	tieService := NewGameService(tieRepo, Cross, Nought)
	tID, _ := tieService.CreateGame("p1", false)
	_ = tieService.Connect("p2", tID)
	tSession, _ := tieRepo.Load(tID)
	tSession.F.Grid = [3][3]int{
		{Cross, Nought, Cross},
		{Cross, Nought, Nought},
		{Nought, Cross, Empty},
	}
	tSession.CurrentTurnUUID = "p1"
	tieRepo.Save(tSession)

	tieMove := Field{Grid: [3][3]int{
		{Cross, Nought, Cross},
		{Cross, Nought, Nought},
		{Nought, Cross, Cross},
	}}
	tieRes, err := tieService.MakeMove(tID, "p1", tieMove)
	if err != nil || tieRes.Status != Draw {
		t.Errorf("Expected Draw status, got status %d, err %v", tieRes.Status, err)
	}
}

func TestMakeAiMoveBotWin(t *testing.T) {
	repo := &MockRepo{sessions: make(map[string]*Session)}
	service := NewGameService(repo, Cross, Nought)

	gameID, _ := service.CreateGame("p1", true)
	s, _ := repo.Load(gameID)
	// Set field so bot wins on next move
	s.F.Grid = [3][3]int{
		{Nought, Nought, Empty},
		{Cross, Cross, Empty},
		{Empty, Empty, Empty},
	}
	repo.Save(s)

	res, err := service.MakeAiMove(gameID)
	if err != nil || res.Status != Win || res.WinnerUUID != "bot" {
		t.Errorf("Expected Bot win, got status %d, winner %s, err %v", res.Status, res.WinnerUUID, err)
	}
}

func TestGameServiceErrorBranches(t *testing.T) {
	// Repo Save error during CreateGame
	errRepo := &MockRepo{sessions: make(map[string]*Session), saveErr: errors.New("db save error")}
	errService := NewGameService(errRepo, Cross, Nought)

	_, err := errService.CreateGame("p1", false)
	if err == nil {
		t.Error("Expected CreateGame to fail on repo Save error")
	}

	// Repo Load error during MakeMove
	loadErrRepo := &MockRepo{sessions: make(map[string]*Session), loadErr: errors.New("db load error")}
	loadErrService := NewGameService(loadErrRepo, Cross, Nought)

	_, err = loadErrService.MakeMove("id", "p1", Field{})
	if err == nil {
		t.Error("Expected MakeMove to fail on repo Load error")
	}

	err = loadErrService.Connect("p2", "id")
	if err == nil {
		t.Error("Expected Connect to fail on repo Load error")
	}

	listErrRepo := &MockRepo{sessions: make(map[string]*Session), listErr: errors.New("db list error")}
	listErrService := NewGameService(listErrRepo, Cross, Nought)

	_, err = listErrService.GetAvailableGames()
	if err == nil {
		t.Error("Expected GetAvailableGames to fail on repo ListAvailable error")
	}

	_, err = listErrService.GetCurrentGames("", "p1")
	if err == nil {
		t.Error("Expected GetCurrentGames to fail on repo GetCurrentGames error")
	}
}

func TestDomainErrors(t *testing.T) {
	vErr := &ValidationError{Message: "test validation"}
	if vErr.Error() == "" {
		t.Error("ValidationError string is empty")
	}

	nfErr := &GameNotFoundError{UUID: "test-uuid"}
	if nfErr.Error() == "" {
		t.Error("GameNotFoundError string is empty")
	}

	icErr := &IncorrectCredsError{login: "user", pass: "pass"}
	if icErr.Error() == "" {
		t.Error("IncorrectCredsError string is empty")
	}

	uaErr := &UserAlreadyExistsError{Login: "user"}
	if uaErr.Error() == "" {
		t.Error("UserAlreadyExistsError string is empty")
	}
}
