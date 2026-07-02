package persistence

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/josephsae/colombia-ecosystems-engine/content"
	"github.com/josephsae/colombia-ecosystems-engine/domain"
	"github.com/josephsae/colombia-ecosystems-engine/engine"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	catalog, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	state, err := engine.NewGame(catalog, domain.NewGameOptions{Seed: 99})
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	path := filepath.Join(t.TempDir(), "partida.json")
	if err := Save(path, state); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path, catalog)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(state, loaded) {
		t.Fatal("loaded state differs from saved state")
	}
}
