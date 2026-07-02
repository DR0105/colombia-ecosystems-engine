package integration_test

import (
	"reflect"
	"testing"

	"github.com/josephsae/colombia-ecosystems-engine/content"
	"github.com/josephsae/colombia-ecosystems-engine/domain"
	"github.com/josephsae/colombia-ecosystems-engine/engine"
	"github.com/josephsae/colombia-ecosystems-engine/persistence"
)

func TestPublicGameFlowAndPersistence(t *testing.T) {
	catalog, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	state, err := engine.NewGame(catalog, domain.NewGameOptions{Seed: 42})
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	if !contains(state.Cards.Hand, "extractive_expansion") {
		t.Fatalf("seeded hand does not contain expected card: %v", state.Cards.Hand)
	}

	played, err := engine.Apply(state, domain.Command{Type: domain.PlayCard, CardID: "extractive_expansion"}, catalog)
	if err != nil {
		t.Fatalf("play card: %v", err)
	}
	if played.State.Resources.Money != 4 || played.State.Environment.Deforestation != 460 {
		t.Fatalf("unexpected state after card: resources=%+v deforestation=%d", played.State.Resources, played.State.Environment.Deforestation)
	}

	ended, err := engine.Apply(played.State, domain.Command{Type: domain.EndTurn}, catalog)
	if err != nil {
		t.Fatalf("end turn: %v", err)
	}
	if ended.State.Round != 1 || len(ended.State.Cards.Hand) != 5 {
		t.Fatalf("unexpected round state: round=%d hand=%d", ended.State.Round, len(ended.State.Cards.Hand))
	}

	path := t.TempDir() + "/partida.json"
	if err := persistence.Save(path, ended.State); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := persistence.Load(path, catalog)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(ended.State, loaded) {
		t.Fatal("loaded state differs from the state produced by the engine")
	}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
