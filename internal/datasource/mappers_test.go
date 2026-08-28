package datasource

import (
	"game-api/internal/domain"
	"testing"
)

func TestDatasourceMappers(t *testing.T) {
	session := &domain.Session{
		UUID:            "game-uuid",
		F:               domain.Field{Grid: [3][3]int{{1, 0, 0}, {0, 2, 0}, {0, 0, 0}}},
		Player1UUID:     "p1",
		Player2UUID:     "p2",
		CurrentTurnUUID: "p1",
		Status:          1,
		IsWithBot:       false,
		WinnerUUID:      "p1",
		Player1Sign:     1,
		Player2Sign:     2,
	}

	model := FromDomain(session)
	if model.UUID != session.UUID || *model.Player1UUID != "p1" || *model.Player2UUID != "p2" || *model.WinnerUUID != "p1" {
		t.Errorf("FromDomain failed: %+v", model)
	}

	domainConverted := ToDomain(&model)
	if domainConverted.UUID != session.UUID || domainConverted.F.Grid != session.F.Grid {
		t.Errorf("ToDomain failed: %+v", domainConverted)
	}

	// Empty string pointer handling in ToDomain
	emptyModel := GameModel{UUID: "empty-uuid"}
	emptyDomain := ToDomain(&emptyModel)
	if emptyDomain.Player1UUID != "" || emptyDomain.Player2UUID != "" || emptyDomain.WinnerUUID != "" {
		t.Errorf("ToDomain nil handling failed: %+v", emptyDomain)
	}

	// User Mappers
	user := &domain.User{
		UUID:         "user-uuid",
		Login:        "alice",
		PasswordHash: "hashedpass",
	}

	userModel := UserFromDomain(user)
	if userModel.UUID != user.UUID || userModel.Login != user.Login {
		t.Errorf("UserFromDomain failed: %+v", userModel)
	}

	userDomain := UserToDomain(&userModel)
	if userDomain.UUID != user.UUID || userDomain.Login != user.Login {
		t.Errorf("UserToDomain failed: %+v", userDomain)
	}
}
