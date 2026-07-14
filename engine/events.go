package engine

import (
	"fmt"
	"sort"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

func resolveEvent(state *domain.GameState, eventID string, catalog domain.Catalog, emitted *[]domain.DomainEvent) error {
	index := activeEventIndex(*state, eventID)
	if index < 0 {
		return ErrEventNotActive
	}
	definition, ok := catalog.Events[eventID]
	if !ok {
		return fmt.Errorf("unknown event %q", eventID)
	}
	if definition.Terminal {
		return fmt.Errorf("terminal event cannot be solved")
	}
	if !hasResources(state.Resources, definition.Solution) {
		return ErrInsufficientResources
	}
	spendResources(&state.Resources, definition.Solution)
	for _, effect := range definition.SolveEffects {
		applyEffect(state, effect, emitted)
	}
	state.Events.Active = append(state.Events.Active[:index], state.Events.Active[index+1:]...)
	state.Events.Resolved = append(state.Events.Resolved, eventID)
	activateQueuedEvents(state, catalog, emitted)
	*emitted = append(*emitted, domain.DomainEvent{Type: "event_solved", Message: "Evento resuelto: " + definition.Name})
	return nil
}

func checkTippingPoints(state *domain.GameState, catalog domain.Catalog, emitted *[]domain.DomainEvent) {
	for _, point := range catalog.TippingPoints {
		if state.Environment.Deforestation < point.Threshold || thresholdCrossed(state.Environment.TippingPointsCrossed, point.Threshold) {
			continue
		}
		state.Environment.TippingPointsCrossed = append(state.Environment.TippingPointsCrossed, point.Threshold)
		spawnEvent(state, point.EventID, catalog, emitted)
	}
}

func thresholdCrossed(points []int, target int) bool {
	for _, point := range points {
		if point == target {
			return true
		}
	}
	return false
}

func spawnEvent(state *domain.GameState, eventID string, catalog domain.Catalog, emitted *[]domain.DomainEvent) bool {
	definition, ok := catalog.Events[eventID]
	if !ok || eventExists(*state, eventID) || isEventBlocked(*state, definition, catalog) {
		return false
	}
	if definition.CooldownRounds > 0 {
		state.Events.Cooldowns[eventID] = definition.CooldownRounds
	}
	rules, err := RulesFor(*state, catalog)
	if err != nil {
		return false
	}
	if definition.Terminal || len(state.Events.Active) < rules.MaxActiveEvents {
		state.Events.Active = append(state.Events.Active, domain.ActiveEvent{ID: eventID, RoundsRemaining: definition.InitialRounds})
		*emitted = append(*emitted, domain.DomainEvent{Type: "event_spawned", Message: "Evento activado: " + definition.Name})
		return true
	}
	state.Events.Queued = append(state.Events.Queued, domain.QueuedEvent{ID: eventID, Priority: eventPriority(definition.Category)})
	*emitted = append(*emitted, domain.DomainEvent{Type: "event_queued", Message: "Evento en espera: " + definition.Name})
	return true
}

func eventPriority(category domain.EventCategory) int {
	switch category {
	case domain.Terminal:
		return 0
	case domain.TippingPoint:
		return 1
	case domain.Consequence:
		return 2
	case domain.BadGovernance:
		return 3
	default:
		return 4
	}
}

func eventExists(state domain.GameState, eventID string) bool {
	if activeEventIndex(state, eventID) >= 0 {
		return true
	}
	for _, event := range state.Events.Queued {
		if event.ID == eventID {
			return true
		}
	}
	return false
}

func activeEventIndex(state domain.GameState, eventID string) int {
	for i, event := range state.Events.Active {
		if event.ID == eventID {
			return i
		}
	}
	return -1
}

func isEventBlocked(state domain.GameState, event domain.EventDefinition, catalog domain.Catalog) bool {
	for _, cardID := range activeCardIDs(state) {
		for _, effect := range catalog.Cards[cardID].Effects {
			if effect.Trigger != domain.Passive {
				continue
			}
			if effect.Type == domain.BlockEvent && effect.EventID == event.ID {
				return true
			}
			if effect.Type == domain.BlockEventCategory && effect.Category == event.Category {
				return true
			}
		}
	}
	return false
}

func advanceEvents(state *domain.GameState, catalog domain.Catalog, emitted *[]domain.DomainEvent) {
	remaining := make([]domain.ActiveEvent, 0, len(state.Events.Active))
	for _, active := range state.Events.Active {
		active.RoundsRemaining--
		if active.RoundsRemaining > 0 {
			remaining = append(remaining, active)
			continue
		}
		definition := catalog.Events[active.ID]
		paidAll := payPartial(&state.Resources, definition.ExpirationLoss)
		if !paidAll {
			for _, effect := range definition.CannotPayEffects {
				applyEffect(state, effect, emitted)
			}
		}
		for _, effect := range definition.ExpirationEffects {
			applyEffect(state, effect, emitted)
		}
		if definition.ID == "illegal_logging" || definition.ID == "land_tenure_conflict" {
			state.Events.TerritorialFailures++
		}
		*emitted = append(*emitted, domain.DomainEvent{Type: "event_expired", Message: "Evento expirado: " + definition.Name})
		if definition.Recurring && !state.Defeat.GameOver {
			remaining = append(remaining, domain.ActiveEvent{ID: active.ID, RoundsRemaining: definition.InitialRounds})
		} else {
			state.Events.Resolved = append(state.Events.Resolved, active.ID)
		}
	}
	state.Events.Active = remaining
	activateQueuedEvents(state, catalog, emitted)
}

func payPartial(current *domain.Resources, required domain.Resources) bool {
	paidAll := true
	if current.Money < required.Money {
		paidAll = false
		current.Money = 0
	} else {
		current.Money -= required.Money
	}
	if current.People < required.People {
		paidAll = false
		current.People = 0
	} else {
		current.People -= required.People
	}
	if current.Land < required.Land {
		paidAll = false
		current.Land = 0
	} else {
		current.Land -= required.Land
	}
	return paidAll
}

func activateQueuedEvents(state *domain.GameState, catalog domain.Catalog, emitted *[]domain.DomainEvent) {
	rules, err := RulesFor(*state, catalog)
	if err != nil {
		return
	}
	sort.SliceStable(state.Events.Queued, func(i, j int) bool {
		return state.Events.Queued[i].Priority < state.Events.Queued[j].Priority
	})
	for len(state.Events.Queued) > 0 && len(state.Events.Active) < rules.MaxActiveEvents {
		queued := state.Events.Queued[0]
		state.Events.Queued = state.Events.Queued[1:]
		definition := catalog.Events[queued.ID]
		if isEventBlocked(*state, definition, catalog) || activeEventIndex(*state, queued.ID) >= 0 {
			continue
		}
		state.Events.Active = append(state.Events.Active, domain.ActiveEvent{ID: queued.ID, RoundsRemaining: definition.InitialRounds})
		*emitted = append(*emitted, domain.DomainEvent{Type: "event_spawned", Message: "Evento activado desde la cola: " + definition.Name})
	}
}

func spawnRandomEvent(state *domain.GameState, catalog domain.Catalog, emitted *[]domain.DomainEvent) {
	rules, err := RulesFor(*state, catalog)
	if err != nil {
		return
	}
	for _, id := range catalog.EventOrder {
		definition := catalog.Events[id]
		if definition.Category != domain.RandomEvent || !eventEligible(*state, definition) {
			continue
		}
		probability := effectiveEventProbability(*state, definition, rules, catalog)
		if nextFloat64(state) < probability && spawnEvent(state, id, catalog, emitted) {
			return
		}
	}
}

func effectiveEventProbability(state domain.GameState, definition domain.EventDefinition, rules domain.ResolvedRules, catalog domain.Catalog) float64 {
	probability := (definition.BaseProbability + eventModifier(state, definition.ID, catalog)) * rules.RandomEventProbabilityMultiplier
	if probability < 0 {
		return 0
	}
	if probability > 1 {
		return 1
	}
	return probability
}

func eventEligible(state domain.GameState, definition domain.EventDefinition) bool {
	if eventExists(state, definition.ID) || state.Events.Cooldowns[definition.ID] > 0 {
		return false
	}
	if state.Environment.Deforestation < definition.MinDeforestation || state.Resources.People < definition.MinPeople {
		return false
	}
	if definition.RequiresSector != "" && !state.Sectors[definition.RequiresSector].Active {
		return false
	}
	return true
}

func eventModifier(state domain.GameState, eventID string, catalog domain.Catalog) float64 {
	modifier := 0.0
	for _, cardID := range activeCardIDs(state) {
		modifier += catalog.Cards[cardID].EventModifiers[eventID]
	}
	return modifier
}

func spawnBadGovernance(state *domain.GameState, catalog domain.Catalog, emitted *[]domain.DomainEvent) {
	rules, err := RulesFor(*state, catalog)
	if err != nil {
		return
	}
	interval := rules.BadGovernanceRoundInterval
	if interval <= 0 || state.Round%interval != 0 {
		return
	}
	var eligible []string
	for _, id := range catalog.EventOrder {
		definition := catalog.Events[id]
		if definition.Category == domain.BadGovernance && !eventExists(*state, id) && !isEventBlocked(*state, definition, catalog) {
			eligible = append(eligible, id)
		}
	}
	if len(eligible) == 0 {
		return
	}
	selected := eligible[int(nextUint64(state)%uint64(len(eligible)))]
	spawnEvent(state, selected, catalog, emitted)
}
