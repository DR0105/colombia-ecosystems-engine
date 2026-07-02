package content

import "testing"

func TestEmbeddedCatalog(t *testing.T) {
	catalog, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	if got := len(catalog.Cards); got != 30 {
		t.Fatalf("cards = %d, want 30", got)
	}
	if got := len(catalog.Events); got != 12 {
		t.Fatalf("events = %d, want 12", got)
	}
	if got := len(catalog.VictoryRoutes); got != 3 {
		t.Fatalf("victory routes = %d, want 3", got)
	}
	if card := catalog.Cards["livestock"]; card.Name != "Ganaderia" || !card.StartingCard {
		t.Fatalf("unexpected starting card: %+v", card)
	}
	if card := catalog.Cards["intensive_livestock"]; card.Name != "Ganaderia intensiva" {
		t.Fatalf("unexpected normalized card: %+v", card)
	}
	if card := catalog.Cards["sustainable_infrastructure"]; card.Name != "Infraestructura sostenible" {
		t.Fatalf("unexpected normalized card: %+v", card)
	}
	counts := map[string]int{}
	for _, card := range catalog.Cards {
		counts[string(card.Sector)]++
	}
	want := map[string]int{"industry": 7, "population": 6, "territory": 6, "ecosystems": 7, "global": 4}
	for sector, expected := range want {
		if counts[sector] != expected {
			t.Errorf("cards in %s = %d, want %d", sector, counts[sector], expected)
		}
	}
}
