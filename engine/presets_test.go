package engine

import (
	"errors"
	"testing"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

func TestTestPresetsFinishOnSecondRound(t *testing.T) {
	catalog := testCatalog(t)
	tests := []struct {
		preset     string
		difficulty string
		victory    string
		defeat     string
	}{
		{preset: PresetVictoryRestorationRound2, difficulty: "easy", victory: "restoration"},
		{preset: PresetVictoryRestorationRound2, difficulty: "normal", victory: "restoration"},
		{preset: PresetVictoryRestorationRound2, difficulty: "hard", victory: "restoration"},
		{preset: PresetDefeatSocialRound2, difficulty: "hard", defeat: "social_collapse"},
		{preset: PresetDefeatEnvironmentalRound2, difficulty: "easy", defeat: "environmental_collapse"},
		{preset: PresetDefeatTerritorialRound2, difficulty: "normal", defeat: "territorial_collapse"},
	}
	for _, test := range tests {
		t.Run(test.preset+"_"+test.difficulty, func(t *testing.T) {
			state, err := NewGame(catalog, domain.NewGameOptions{Seed: 42, DifficultyID: test.difficulty, TestPresetID: test.preset})
			if err != nil {
				t.Fatal(err)
			}
			if state.TestPresetID != test.preset || state.Round != 0 || len(state.Cards.Hand) != 4 {
				t.Fatalf("unexpected preset state: %+v", state)
			}
			first, err := Apply(state, domain.Command{Type: domain.EndTurn}, catalog)
			if err != nil {
				t.Fatal(err)
			}
			if first.State.Round != 1 || first.State.Phase != domain.Decision || first.State.Victory.Completed || first.State.Defeat.GameOver {
				t.Fatalf("preset ended before round 2: %+v", first.State)
			}
			second, err := Apply(first.State, domain.Command{Type: domain.EndTurn}, catalog)
			if err != nil {
				t.Fatal(err)
			}
			if second.State.Round != 2 || second.State.Phase != domain.Finished {
				t.Fatalf("preset did not finish on round 2: %+v", second.State)
			}
			if test.victory != "" && (!second.State.Victory.Completed || second.State.Victory.Route != test.victory) {
				t.Fatalf("victory = %+v, want %s", second.State.Victory, test.victory)
			}
			if test.defeat != "" && (!second.State.Defeat.GameOver || second.State.Defeat.Reason != test.defeat) {
				t.Fatalf("defeat = %+v, want %s", second.State.Defeat, test.defeat)
			}
		})
	}
}

func TestUnknownTestPresetIsRejected(t *testing.T) {
	catalog := testCatalog(t)
	_, err := NewGame(catalog, domain.NewGameOptions{Seed: 42, TestPresetID: "unknown"})
	if !errors.Is(err, ErrInvalidTestPreset) {
		t.Fatalf("error = %v, want ErrInvalidTestPreset", err)
	}
}
