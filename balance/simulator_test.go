package balance

import (
	"reflect"
	"testing"

	"github.com/josephsae/colombia-ecosystems-engine/content"
)

func TestSimulationIsDeterministicAndCoversAllDifficulties(t *testing.T) {
	catalog, err := content.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	config := Config{GamesPerDifficulty: 6, MaxRounds: 60, FirstSeed: 100}
	first, err := Run(catalog, config)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(catalog, config)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same simulation configuration produced different reports")
	}
	if len(first.Results) != 3 {
		t.Fatalf("results = %d, want 3", len(first.Results))
	}
	for _, result := range first.Results {
		if result.Victories+result.Defeats+result.Timeouts != config.GamesPerDifficulty {
			t.Fatalf("incomplete report for %s: %+v", result.Difficulty, result)
		}
	}
}
