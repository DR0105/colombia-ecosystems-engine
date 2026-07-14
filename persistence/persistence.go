package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

func Save(path string, state domain.GameState) error {
	if path == "" {
		return fmt.Errorf("save path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create save directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode save: %w", err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write save: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("commit save: %w", err)
	}
	return nil
}

func Load(path string, catalog domain.Catalog) (domain.GameState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.GameState{}, fmt.Errorf("read save: %w", err)
	}
	var state domain.GameState
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.GameState{}, fmt.Errorf("decode save: %w", err)
	}
	if err := ValidateState(state, catalog); err != nil {
		return domain.GameState{}, err
	}
	if state.DifficultyID == "" {
		state.DifficultyID = domain.LegacyDifficultyID
	}
	return state, nil
}

func ValidateState(state domain.GameState, catalog domain.Catalog) error {
	if state.SchemaVersion != catalog.Scenario.SchemaVersion {
		return fmt.Errorf("unsupported schema version %d", state.SchemaVersion)
	}
	if state.ScenarioID != catalog.Scenario.ID {
		return fmt.Errorf("unknown scenario %q", state.ScenarioID)
	}
	difficultyID := state.DifficultyID
	if difficultyID == "" {
		difficultyID = domain.LegacyDifficultyID
	}
	if _, ok := catalog.Difficulties[difficultyID]; !ok {
		return fmt.Errorf("unknown difficulty %q", difficultyID)
	}
	for _, id := range append(append(append([]string{}, state.Cards.Hand...), state.Cards.Deck...), state.Cards.Discard...) {
		if _, ok := catalog.Cards[id]; !ok {
			return fmt.Errorf("save references unknown card %q", id)
		}
	}
	for _, active := range state.Events.Active {
		if _, ok := catalog.Events[active.ID]; !ok {
			return fmt.Errorf("save references unknown event %q", active.ID)
		}
	}
	if state.RNGState == 0 {
		return fmt.Errorf("save has invalid rng state")
	}
	return nil
}
