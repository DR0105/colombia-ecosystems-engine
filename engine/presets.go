package engine

import (
	"fmt"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

const (
	PresetVictoryRestorationRound2  = "victory_restoration_round_2"
	PresetDefeatSocialRound2        = "defeat_social_round_2"
	PresetDefeatEnvironmentalRound2 = "defeat_environmental_round_2"
	PresetDefeatTerritorialRound2   = "defeat_territorial_round_2"
)

var testPresetIDs = []string{
	PresetVictoryRestorationRound2,
	PresetDefeatSocialRound2,
	PresetDefeatEnvironmentalRound2,
	PresetDefeatTerritorialRound2,
}

func TestPresetIDs() []string {
	return append([]string(nil), testPresetIDs...)
}

func validTestPreset(id string) bool {
	for _, candidate := range testPresetIDs {
		if id == candidate {
			return true
		}
	}
	return false
}

func applyTestPreset(state *domain.GameState, presetID string, catalog domain.Catalog) error {
	resetForTestPreset(state)
	state.TestPresetID = presetID

	switch presetID {
	case PresetVictoryRestorationRound2:
		if err := configureRestorationVictory(state, catalog); err != nil {
			return err
		}
	case PresetDefeatSocialRound2:
		rules, err := RulesFor(*state, catalog)
		if err != nil {
			return err
		}
		state.Resources = domain.Resources{Money: 1, People: 1, Land: 1}
		state.Environment.Deforestation = 100
		state.SocialPressure = rules.SocialPressureLimit - 1
		state.Events.Active = []domain.ActiveEvent{{ID: "armed_conflict", RoundsRemaining: 2}}
	case PresetDefeatEnvironmentalRound2:
		if err := configureEnvironmentalDefeat(state, catalog); err != nil {
			return err
		}
	case PresetDefeatTerritorialRound2:
		rules, err := RulesFor(*state, catalog)
		if err != nil {
			return err
		}
		state.Resources = domain.Resources{Money: 1, People: 1, Land: 1}
		state.Environment.Deforestation = 100
		state.Events.TerritorialFailures = rules.TerritorialFailureLimit - 1
		state.Events.Active = []domain.ActiveEvent{{ID: "illegal_logging", RoundsRemaining: 2}}
	default:
		return fmt.Errorf("%w: %s", ErrInvalidTestPreset, presetID)
	}

	setPresetHandSize(state, 4)
	return nil
}

func resetForTestPreset(state *domain.GameState) {
	state.Round = 0
	state.Phase = domain.Decision
	state.Resources = domain.Resources{}
	state.Environment = domain.EnvironmentState{}
	state.SocialPressure = 0
	state.Victory = domain.VictoryState{}
	state.Defeat = domain.DefeatState{}
	state.Events = domain.EventState{Cooldowns: make(map[string]int)}
	state.Cards.Discard = nil
	state.Cards.Policies = nil
	state.Cards.Projects = nil
	state.Cards.Milestones = make(map[string]bool)
	for id, sector := range state.Sectors {
		sector.Active = false
		sector.CycleProgress = 0
		sector.ActiveCards = nil
		state.Sectors[id] = sector
	}
}

func configureRestorationVictory(state *domain.GameState, catalog domain.Catalog) error {
	var route domain.VictoryRoute
	found := false
	for _, candidate := range catalog.VictoryRoutes {
		if candidate.ID == "restoration" {
			route, found = candidate, true
			break
		}
	}
	if !found {
		return fmt.Errorf("restoration victory route is missing")
	}
	difficulty := catalog.Difficulties[state.DifficultyID]
	minCards := min(len(route.RequiredCards), max(1, route.MinCards+difficulty.VictoryModifiers.MinCards))
	minLand := max(1, route.MinLand+difficulty.VictoryModifiers.MinLand)
	state.Resources = domain.Resources{Money: 2, People: max(1, route.MinPeople+difficulty.VictoryModifiers.MinPeople), Land: minLand - 1}
	state.Environment.Deforestation = 500

	for _, cardID := range route.RequiredCards[:minCards] {
		activatePresetCard(state, catalog.Cards[cardID])
	}

	territory := state.Sectors[domain.Territory]
	territory.Active = true
	definition := catalog.Sectors[domain.Territory]
	advance := definition.BaseAdvance
	for _, cardID := range territory.ActiveCards {
		advance += catalog.Cards[cardID].Arrows
	}
	advance = min(advance, definition.BaseAdvance+catalog.Scenario.MaxEffectiveArrows)
	territory.CycleProgress = definition.CycleTarget - 2*advance
	if territory.CycleProgress < 0 {
		return fmt.Errorf("territory advances too quickly for round 2 preset")
	}
	state.Sectors[domain.Territory] = territory
	return nil
}

func configureEnvironmentalDefeat(state *domain.GameState, catalog domain.Catalog) error {
	card := catalog.Cards["livestock"]
	activatePresetCard(state, card)
	industry := state.Sectors[domain.Industry]
	industry.Active = true
	definition := catalog.Sectors[domain.Industry]
	advance := definition.BaseAdvance + min(card.Arrows, catalog.Scenario.MaxEffectiveArrows)
	industry.CycleProgress = definition.CycleTarget - 2*advance
	if industry.CycleProgress < 0 {
		return fmt.Errorf("industry advances too quickly for round 2 preset")
	}
	state.Sectors[domain.Industry] = industry

	impact := definition.EnvironmentImpact
	for _, effect := range card.Effects {
		if effect.Trigger == domain.Passive && effect.Type == domain.ModifySectorEnvironmentImpact && effect.Sector == domain.Industry {
			impact += effect.Amount
		}
	}
	if impact <= 0 {
		return fmt.Errorf("industry must have positive environmental impact for preset")
	}
	state.Resources = domain.Resources{Money: 1, People: 1, Land: 1}
	state.Environment = domain.EnvironmentState{
		Deforestation:        3000 - impact,
		TippingPointsCrossed: []int{500, 1500, 2000, 2500},
	}
	return nil
}

func activatePresetCard(state *domain.GameState, card domain.CardDefinition) {
	removePresetCard(state, card.ID)
	state.Cards.Milestones[card.ID] = true
	switch card.Type {
	case domain.Structure:
		sector := state.Sectors[card.Sector]
		sector.ActiveCards = append(sector.ActiveCards, card.ID)
		state.Sectors[card.Sector] = sector
	case domain.Policy:
		state.Cards.Policies = append(state.Cards.Policies, card.ID)
	case domain.Project:
		state.Cards.Projects = append(state.Cards.Projects, card.ID)
	}
}

func removePresetCard(state *domain.GameState, cardID string) {
	state.Cards.Hand = removeValue(state.Cards.Hand, cardID)
	state.Cards.Deck = removeValue(state.Cards.Deck, cardID)
	state.Cards.Discard = removeValue(state.Cards.Discard, cardID)
	state.Cards.Policies = removeValue(state.Cards.Policies, cardID)
	state.Cards.Projects = removeValue(state.Cards.Projects, cardID)
	for id, sector := range state.Sectors {
		sector.ActiveCards = removeValue(sector.ActiveCards, cardID)
		state.Sectors[id] = sector
	}
}

func removeValue(values []string, target string) []string {
	result := values[:0]
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func setPresetHandSize(state *domain.GameState, size int) {
	for len(state.Cards.Hand) > size {
		last := len(state.Cards.Hand) - 1
		state.Cards.Deck = append(state.Cards.Deck, state.Cards.Hand[last])
		state.Cards.Hand = state.Cards.Hand[:last]
	}
	if len(state.Cards.Hand) < size {
		drawCards(state, size-len(state.Cards.Hand))
	}
}
