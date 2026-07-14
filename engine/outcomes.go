package engine

import "github.com/josephsae/colombia-ecosystems-engine/domain"

func evaluateOutcome(state *domain.GameState, catalog domain.Catalog, emitted *[]domain.DomainEvent) {
	rules, err := RulesFor(*state, catalog)
	if err != nil {
		setDefeat(state, "invalid_game_state", emitted)
		return
	}
	if state.Defeat.GameOver {
		state.Phase = domain.Finished
		return
	}
	if state.Environment.Deforestation >= 3000 || hasTerminalEvent(*state, catalog) {
		setDefeat(state, "environmental_collapse", emitted)
		return
	}
	if state.SocialPressure >= rules.SocialPressureLimit {
		setDefeat(state, "social_collapse", emitted)
		return
	}
	if state.Events.TerritorialFailures >= rules.TerritorialFailureLimit {
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

func routeCompleted(state domain.GameState, route domain.VictoryRoute, catalog domain.Catalog) bool {
	difficultyID := state.DifficultyID
	if difficultyID == "" {
		difficultyID = domain.LegacyDifficultyID
	}
	modifiers := catalog.Difficulties[difficultyID].VictoryModifiers
	maxDeforestation := route.MaxDeforestation + modifiers.MaxDeforestation
	minPeople := max(0, route.MinPeople+modifiers.MinPeople)
	minLand := max(0, route.MinLand+modifiers.MinLand)
	minCards := min(len(route.RequiredCards), max(1, route.MinCards+modifiers.MinCards))
	if route.MaxExclusive {
		if state.Environment.Deforestation >= maxDeforestation {
			return false
		}
	} else if state.Environment.Deforestation > maxDeforestation {
		return false
	}
	if state.Resources.People < minPeople || state.Resources.Land < minLand {
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
	return completed >= minCards
}
