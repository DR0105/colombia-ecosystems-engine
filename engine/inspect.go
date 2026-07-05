package engine

import (
	"errors"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

func Inspect(state domain.GameState, catalog domain.Catalog) domain.AvailableActions {
	actions := domain.AvailableActions{
		Cards:    make(map[string]domain.ActionAvailability, len(state.Cards.Hand)),
		Events:   make(map[string]domain.ActionAvailability, len(state.Events.Active)),
		Discards: make(map[string]domain.ActionAvailability, len(state.Cards.Hand)),
	}
	for _, cardID := range state.Cards.Hand {
		actions.Cards[cardID] = inspectCommand(state, domain.Command{Type: domain.PlayCard, CardID: cardID}, catalog)
		actions.Discards[cardID] = inspectCommand(state, domain.Command{Type: domain.DiscardCard, CardID: cardID}, catalog)
	}
	for _, active := range state.Events.Active {
		actions.Events[active.ID] = inspectCommand(state, domain.Command{Type: domain.ResolveEvent, EventID: active.ID}, catalog)
	}
	actions.CanEndTurn = inspectCommand(state, domain.Command{Type: domain.EndTurn}, catalog)
	return actions
}

func inspectCommand(state domain.GameState, command domain.Command, catalog domain.Catalog) domain.ActionAvailability {
	_, err := Apply(state, command, catalog)
	if err == nil {
		return domain.ActionAvailability{Allowed: true}
	}
	return domain.ActionAvailability{Allowed: false, Code: ErrorCode(err), Message: err.Error()}
}

func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrGameAlreadyOver):
		return "GAME_ALREADY_OVER"
	case errors.Is(err, ErrCardNotInHand):
		return "CARD_NOT_IN_HAND"
	case errors.Is(err, ErrInsufficientResources):
		return "INSUFFICIENT_RESOURCES"
	case errors.Is(err, ErrSectorInactive):
		return "SECTOR_INACTIVE"
	case errors.Is(err, ErrRequirementsNotMet):
		return "REQUIREMENTS_NOT_MET"
	case errors.Is(err, ErrActiveCardLimit):
		return "ACTIVE_CARD_LIMIT_REACHED"
	case errors.Is(err, ErrEventNotActive):
		return "EVENT_NOT_ACTIVE"
	case errors.Is(err, ErrHandLimit):
		return "HAND_LIMIT_EXCEEDED"
	case errors.Is(err, ErrInvalidCommand):
		return "INVALID_COMMAND"
	default:
		return "ENGINE_ERROR"
	}
}
