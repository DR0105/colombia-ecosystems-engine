package engine

import "github.com/josephsae/colombia-ecosystems-engine/domain"

func evaluateOutcome(state *domain.GameState, catalog domain.Catalog, emitted *[]domain.DomainEvent) {
	if state.Defeat.GameOver {
		state.Phase = domain.Finished
		return
	}
	if state.Environment.Deforestation >= 3000 || hasTerminalEvent(*state, catalog) {
		setDefeat(state, "environmental_collapse", emitted)
		return
	}
	if state.SocialPressure >= 3 || (state.Resources.People == 0 && hasActiveSocialCrisis(*state)) {
		setDefeat(state, "social_collapse", emitted)
		return
	}
	territory := state.Sectors[domain.Territory]
	if state.Events.TerritorialFailures >= 2 || (territory.Active && state.Resources.Land == 0 && activeEventIndex(*state, "land_tenure_conflict") >= 0) {
		setDefeat(state, "territorial_collapse", emitted)
		return
	}
	for _, route := range catalog.VictoryRoutes {
		if routeCompleted(*state, route, catalog) {
			state.Victory = domain.VictoryState{Completed: true, Route: route.ID}
			state.Phase = domain.Finished
			*emitted = append(*emitted, domain.DomainEvent{Type: "victory", Message: "Victoria: " + route.Name})
			return
		}
	}
}

func setDefeat(state *domain.GameState, reason string, emitted *[]domain.DomainEvent) {
	state.Defeat = domain.DefeatState{GameOver: true, Reason: reason}
	state.Phase = domain.Finished
	*emitted = append(*emitted, domain.DomainEvent{Type: "defeat", Message: "Derrota: " + reason})
}

func hasTerminalEvent(state domain.GameState, catalog domain.Catalog) bool {
	for _, active := range state.Events.Active {
		if catalog.Events[active.ID].Terminal {
			return true
		}
	}
	return false
}

func hasActiveSocialCrisis(state domain.GameState) bool {
	for _, id := range []string{"famine", "food_conflict", "armed_conflict", "health_crisis"} {
		if activeEventIndex(state, id) >= 0 {
			return true
		}
	}
	return false
}

func routeCompleted(state domain.GameState, route domain.VictoryRoute, catalog domain.Catalog) bool {
	if route.MaxExclusive {
		if state.Environment.Deforestation >= route.MaxDeforestation {
			return false
		}
	} else if state.Environment.Deforestation > route.MaxDeforestation {
		return false
	}
	if state.Resources.People < route.MinPeople || state.Resources.Land < route.MinLand {
		return false
	}
	if route.NoTerminalEvent && hasTerminalEvent(state, catalog) {
		return false
	}
	completed := 0
	for _, cardID := range route.RequiredCards {
		card := catalog.Cards[cardID]
		if card.Type == domain.Action {
			if state.Cards.Milestones[cardID] {
				completed++
			}
			continue
		}
		if isCardActive(state, cardID) {
			completed++
		}
	}
	return completed >= route.MinCards
}
