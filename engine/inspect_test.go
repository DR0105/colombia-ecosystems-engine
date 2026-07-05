package engine

import (
	"testing"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

func TestInspectReportsAllowedAndBlockedActions(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	state.Cards.Hand = []string{"extractive_expansion", "solar_industry"}

	actions := Inspect(state, catalog)
	if !actions.Cards["extractive_expansion"].Allowed {
		t.Fatalf("extractive expansion should be playable: %+v", actions.Cards["extractive_expansion"])
	}
	if actions.Cards["solar_industry"].Allowed || actions.Cards["solar_industry"].Code != "INSUFFICIENT_RESOURCES" {
		t.Fatalf("solar industry should be blocked by resources first: %+v", actions.Cards["solar_industry"])
	}
	if !actions.Discards["solar_industry"].Allowed || !actions.CanEndTurn.Allowed {
		t.Fatalf("discard and end turn should be allowed: %+v", actions)
	}
}

func TestInspectRequiresDiscardWhenHandIsOverLimit(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	state.Phase = domain.DiscardRequired
	state.Cards.Hand = append(state.Cards.Hand, "community_health", "rural_cadastre", "peace_agreements")

	actions := Inspect(state, catalog)
	if actions.CanEndTurn.Allowed || actions.CanEndTurn.Code != "HAND_LIMIT_EXCEEDED" {
		t.Fatalf("end turn availability = %+v", actions.CanEndTurn)
	}
	for _, cardID := range state.Cards.Hand {
		if !actions.Discards[cardID].Allowed {
			t.Fatalf("discard %s should be allowed: %+v", cardID, actions.Discards[cardID])
		}
	}
}
