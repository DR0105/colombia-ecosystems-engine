package engine

import (
	"fmt"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

func playCard(state *domain.GameState, cardID string, catalog domain.Catalog, emitted *[]domain.DomainEvent) error {
	index := indexOf(state.Cards.Hand, cardID)
	if index < 0 {
		return ErrCardNotInHand
	}
	card, ok := catalog.Cards[cardID]
	if !ok {
		return fmt.Errorf("unknown card %q", cardID)
	}
	if !hasResources(state.Resources, card.Cost) {
		return ErrInsufficientResources
	}
	for _, required := range card.Requires {
		if !state.Cards.Milestones[required] && !isCardActive(*state, required) {
			return fmt.Errorf("%w: %s", ErrRequirementsNotMet, required)
		}
	}
	if card.Sector != domain.Global {
		sector := state.Sectors[card.Sector]
		if !sector.Active && !cardActivatesSector(card, card.Sector) {
			return ErrSectorInactive
		}
	}
	if exceedsActiveLimit(*state, card) {
		return ErrActiveCardLimit
	}

	spendResources(&state.Resources, card.Cost)
	state.Cards.Hand = removeAt(state.Cards.Hand, index)
	state.Cards.Milestones[card.ID] = true
	for _, effect := range card.Effects {
		if effect.Trigger == domain.OnPlay {
			applyEffect(state, effect, emitted)
		}
	}

	switch card.Type {
	case domain.Action:
		state.Cards.Discard = append(state.Cards.Discard, card.ID)
	case domain.Structure:
		sector := state.Sectors[card.Sector]
		sector.ActiveCards = append(sector.ActiveCards, card.ID)
		state.Sectors[card.Sector] = sector
	case domain.Policy:
		state.Cards.Policies = append(state.Cards.Policies, card.ID)
	case domain.Project:
		state.Cards.Projects = append(state.Cards.Projects, card.ID)
	}
	*emitted = append(*emitted, domain.DomainEvent{Type: "card_played", Message: "Carta jugada: " + card.Name})
	return nil
}

func discardCard(state *domain.GameState, cardID string, catalog domain.Catalog, emitted *[]domain.DomainEvent) error {
	index := indexOf(state.Cards.Hand, cardID)
	if index < 0 {
		return ErrCardNotInHand
	}
	state.Cards.Hand = removeAt(state.Cards.Hand, index)
	state.Cards.Discard = append(state.Cards.Discard, cardID)
	if len(state.Cards.Hand) <= catalog.Scenario.HandLimit {
		state.Phase = domain.Decision
	}
	*emitted = append(*emitted, domain.DomainEvent{Type: "card_discarded", Message: "Carta descartada: " + catalog.Cards[cardID].Name})
	return nil
}

func cardActivatesSector(card domain.CardDefinition, sector domain.SectorID) bool {
	for _, effect := range card.Effects {
		if effect.Trigger == domain.OnPlay && effect.Type == domain.ActivateSector && effect.Sector == sector {
			return true
		}
	}
	return false
}

func exceedsActiveLimit(state domain.GameState, card domain.CardDefinition) bool {
	switch card.Type {
	case domain.Structure:
		return len(state.Sectors[card.Sector].ActiveCards) >= 5
	case domain.Policy:
		return len(state.Cards.Policies) >= 6
	default:
		return false
	}
}

func hasResources(current, required domain.Resources) bool {
	return current.Money >= required.Money && current.People >= required.People && current.Land >= required.Land
}

func spendResources(current *domain.Resources, required domain.Resources) {
	current.Money -= required.Money
	current.People -= required.People
	current.Land -= required.Land
}

func isCardActive(state domain.GameState, cardID string) bool {
	for _, sector := range state.Sectors {
		if indexOf(sector.ActiveCards, cardID) >= 0 {
			return true
		}
	}
	return indexOf(state.Cards.Policies, cardID) >= 0 || indexOf(state.Cards.Projects, cardID) >= 0
}

func activeCardIDs(state domain.GameState) []string {
	var ids []string
	for _, sectorID := range []domain.SectorID{domain.Industry, domain.Population, domain.Territory, domain.Ecosystems} {
		ids = append(ids, state.Sectors[sectorID].ActiveCards...)
	}
	ids = append(ids, state.Cards.Policies...)
	ids = append(ids, state.Cards.Projects...)
	return ids
}

func indexOf(values []string, value string) int {
	for i, candidate := range values {
		if candidate == value {
			return i
		}
	}
	return -1
}

func removeAt(values []string, index int) []string {
	return append(values[:index], values[index+1:]...)
}
