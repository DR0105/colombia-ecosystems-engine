package engine

import "github.com/josephsae/colombia-ecosystems-engine/domain"

var sectorOrder = []domain.SectorID{domain.Industry, domain.Population, domain.Territory, domain.Ecosystems}

func endTurn(state *domain.GameState, catalog domain.Catalog, emitted *[]domain.DomainEvent) error {
	if len(state.Cards.Hand) > catalog.Scenario.HandLimit {
		state.Phase = domain.DiscardRequired
		return ErrHandLimit
	}
	state.Round++
	decrementCooldowns(state)
	for _, sectorID := range sectorOrder {
		advanceSector(state, sectorID, catalog, emitted)
	}
	checkTippingPoints(state, catalog, emitted)
	advanceEvents(state, catalog, emitted)
	spawnRandomEvent(state, catalog, emitted)
	spawnBadGovernance(state, catalog, emitted)
	activateQueuedEvents(state, catalog, emitted)
	evaluateOutcome(state, catalog, emitted)
	if !state.Defeat.GameOver && !state.Victory.Completed {
		drawCards(state, catalog.Scenario.DrawPerRound)
		if len(state.Cards.Hand) > catalog.Scenario.HandLimit {
			state.Phase = domain.DiscardRequired
		} else {
			state.Phase = domain.Decision
		}
	}
	*emitted = append(*emitted, domain.DomainEvent{Type: "round_completed", Message: "Ronda resuelta"})
	return nil
}

func advanceSector(state *domain.GameState, sectorID domain.SectorID, catalog domain.Catalog, emitted *[]domain.DomainEvent) {
	sector := state.Sectors[sectorID]
	if !sector.Active {
		return
	}
	definition := catalog.Sectors[sectorID]
	arrows := 0
	for _, cardID := range sector.ActiveCards {
		arrows += catalog.Cards[cardID].Arrows
	}
	if arrows > catalog.Scenario.MaxEffectiveArrows {
		arrows = catalog.Scenario.MaxEffectiveArrows
	}
	sector.CycleProgress += definition.BaseAdvance + arrows
	for sector.CycleProgress >= definition.CycleTarget {
		sector.CycleProgress -= definition.CycleTarget
		state.Sectors[sectorID] = sector
		produceSector(state, sectorID, catalog, emitted)
		sector = state.Sectors[sectorID]
	}
	state.Sectors[sectorID] = sector
}

func produceSector(state *domain.GameState, sectorID domain.SectorID, catalog domain.Catalog, emitted *[]domain.DomainEvent) {
	if sectorID == domain.Population && state.Resources.Money < 1 {
		if eventExists(*state, "famine") {
			state.SocialPressure++
			*emitted = append(*emitted, domain.DomainEvent{Type: "famine_aggravated", Message: "La hambruna aumento la presion social"})
		} else {
			spawnEvent(state, "famine", catalog, emitted)
		}
		return
	}
	if sectorID == domain.Population {
		state.Resources.Money--
	}

	definition := catalog.Sectors[sectorID]
	production := definition.Production
	impact := definition.EnvironmentImpact
	for _, cardID := range activeCardIDs(*state) {
		card := catalog.Cards[cardID]
		for _, effect := range card.Effects {
			if effect.Trigger != domain.Passive || effect.Sector != sectorID {
				continue
			}
			switch effect.Type {
			case domain.ModifySectorProduction:
				changeResource(&production, effect.Resource, effect.Amount)
			case domain.ModifySectorEnvironmentImpact:
				impact += effect.Amount
			}
		}
	}
	state.Resources.Money += production.Money
	state.Resources.People += production.People
	state.Resources.Land += production.Land
	state.Environment.Deforestation += impact

	for _, cardID := range activeCardIDs(*state) {
		for _, effect := range catalog.Cards[cardID].Effects {
			if effect.Trigger == domain.OnSectorProduction && effect.Sector == sectorID {
				applyEffect(state, effect, emitted)
			}
		}
	}
	*emitted = append(*emitted, domain.DomainEvent{Type: "sector_produced", Message: "Produccion de " + definition.Name})
}

func decrementCooldowns(state *domain.GameState) {
	for id, rounds := range state.Events.Cooldowns {
		if rounds <= 1 {
			delete(state.Events.Cooldowns, id)
		} else {
			state.Events.Cooldowns[id] = rounds - 1
		}
	}
}
