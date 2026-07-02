package engine

import "github.com/josephsae/colombia-ecosystems-engine/domain"

func applyEffect(state *domain.GameState, effect domain.Effect, emitted *[]domain.DomainEvent) {
	switch effect.Type {
	case domain.GainResource:
		changeResource(&state.Resources, effect.Resource, effect.Amount)
	case domain.LoseResource:
		changeResource(&state.Resources, effect.Resource, -effect.Amount)
	case domain.AddDeforestation:
		state.Environment.Deforestation += effect.Amount
	case domain.RemoveDeforestation:
		state.Environment.Deforestation -= effect.Amount
	case domain.ActivateSector:
		sector := state.Sectors[effect.Sector]
		sector.Active = true
		state.Sectors[effect.Sector] = sector
	case domain.AddSocialPressure:
		state.SocialPressure += effect.Amount
	case domain.RemoveSocialPressure:
		state.SocialPressure -= effect.Amount
		if state.SocialPressure < 0 {
			state.SocialPressure = 0
		}
	case domain.CompleteProjectFlag:
		state.Cards.Milestones[effect.Flag] = true
	case domain.TriggerDefeat:
		state.Defeat = domain.DefeatState{GameOver: true, Reason: effect.Reason}
		*emitted = append(*emitted, domain.DomainEvent{Type: "defeat_triggered", Message: "Derrota: " + effect.Reason})
	case domain.ModifySectorProduction, domain.ModifySectorEnvironmentImpact,
		domain.BlockEvent, domain.BlockEventCategory, domain.ModifyEventProbability:
		// Passive effects are calculated from active cards when their target system runs.
	}
}

func changeResource(resources *domain.Resources, id domain.ResourceID, amount int) {
	switch id {
	case domain.Money:
		resources.Money += amount
	case domain.People:
		resources.People += amount
	case domain.Land:
		resources.Land += amount
	}
}
