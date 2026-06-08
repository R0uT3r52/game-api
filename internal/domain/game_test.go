package domain

import (
	"testing"
)

func TestEvaluate(t *testing.T) {
	service := &GameService{
		PlayerSign: Cross,
		BotSign:    Nought,
	}
	PlayerSign := service.PlayerSign
	BotSign := service.BotSign

	tests := []struct {
		name  string
		field Field
		want  int
	}{
		{
			name: "BotSign wins row",
			field: Field{
				Grid: [3][3]int{
					{BotSign, BotSign, BotSign},
					{Empty, Empty, Empty},
					{Empty, Empty, Empty},
				},
			},
			want: 10,
		},
		{
			name: "PlayerSign wins column",
			field: Field{
				Grid: [3][3]int{
					{PlayerSign, Empty, Empty},
					{PlayerSign, Empty, Empty},
					{PlayerSign, Empty, Empty},
				},
			},
			want: -10,
		},
		{
			name: "BotSign wins diagonal",
			field: Field{
				Grid: [3][3]int{
					{BotSign, Empty, Empty},
					{Empty, BotSign, Empty},
					{Empty, Empty, BotSign},
				},
			},
			want: 10,
		},
		{
			name: "Draw",
			field: Field{
				Grid: [3][3]int{
					{BotSign, PlayerSign, BotSign},
					{BotSign, PlayerSign, PlayerSign},
					{PlayerSign, BotSign, PlayerSign},
				},
			},
			want: 0,
		},
		{
			name: "Empty field",
			field: Field{
				Grid: [3][3]int{
					{Empty, Empty, Empty},
					{Empty, Empty, Empty},
					{Empty, Empty, Empty},
				},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := service.evaluate(&tt.field); got != tt.want {
				t.Errorf("evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetNextMove(t *testing.T) {
	service := &GameService{
		PlayerSign: Cross,
		BotSign:    Nought,
	}
	PlayerSign := service.PlayerSign
	BotSign := service.BotSign

	tests := []struct {
		name    string
		field   Field
		wantRow int
		wantCol int
	}{
		{
			name: "BotSign should win in a row",
			field: Field{
				Grid: [3][3]int{
					{BotSign, BotSign, Empty},
					{PlayerSign, Empty, Empty},
					{PlayerSign, Empty, Empty},
				},
			},
			wantRow: 0,
			wantCol: 2,
		},
		{
			name: "BotSign should win in a column",
			field: Field{
				Grid: [3][3]int{
					{BotSign, PlayerSign, Empty},
					{BotSign, Empty, Empty},
					{Empty, PlayerSign, Empty},
				},
			},
			wantRow: 2,
			wantCol: 0,
		},
		{
			name: "BotSign should win in a diagonal",
			field: Field{
				Grid: [3][3]int{
					{BotSign, PlayerSign, Empty},
					{Empty, BotSign, Empty},
					{PlayerSign, Empty, Empty},
				},
			},
			wantRow: 2,
			wantCol: 2,
		},
		{
			name: "BotSign should block PlayerSign row",
			field: Field{
				Grid: [3][3]int{
					{PlayerSign, PlayerSign, Empty},
					{Empty, BotSign, Empty},
					{Empty, Empty, Empty},
				},
			},
			wantRow: 0,
			wantCol: 2,
		},
		{
			name: "BotSign should block PlayerSign column",
			field: Field{
				Grid: [3][3]int{
					{PlayerSign, Empty, Empty},
					{PlayerSign, BotSign, Empty},
					{Empty, Empty, Empty},
				},
			},
			wantRow: 2,
			wantCol: 0,
		},
		{
			name: "BotSign should block PlayerSign diagonal",
			field: Field{
				Grid: [3][3]int{
					{PlayerSign, Empty, Empty},
					{Empty, PlayerSign, Empty},
					{Empty, Empty, Empty},
				},
			},
			wantRow: 2,
			wantCol: 2,
		},
		{
			name: "BotSign should prioritize win over block",
			field: Field{
				Grid: [3][3]int{
					{BotSign, BotSign, Empty},
					{PlayerSign, PlayerSign, Empty},
					{Empty, Empty, Empty},
				},
			},
			wantRow: 0,
			wantCol: 2,
		},
		{
			name: "BotSign should move on empty field",
			field: Field{
				Grid: [3][3]int{},
			},
			wantRow: 0,
			wantCol: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRow, gotCol := service.getNextMove(&tt.field)
			if gotRow != tt.wantRow || gotCol != tt.wantCol {
				t.Errorf("GetNextMove() = (%v, %v), want (%v, %v)", gotRow, gotCol, tt.wantRow, tt.wantCol)
			}
		})
	}
}
