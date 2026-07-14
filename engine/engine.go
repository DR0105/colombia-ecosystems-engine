package engine

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

var (
	ErrGameAlreadyOver       = errors.New("game already over")
	ErrCardNotInHand         = errors.New("card not in hand")
	ErrInsufficientResources = errors.New("insufficient resources")
	ErrSectorInactive        = errors.New("sector inactive")
	ErrRequirementsNotMet    = errors.New("requirements not met")
	ErrActiveCardLimit       = errors.New("active card limit reached")
	ErrEventNotActive        = errors.New("event not active")
	ErrHandLimit             = errors.New("hand limit exceeded")
	ErrInvalidCommand        = errors.New("invalid command")
	ErrInvalidDifficulty     = errors.New("invalid difficulty")
)

func NewGame(catalog domain.Catalog, options domain.NewGameOptions) (domain.GameState, error) {
	difficultyID := options.DifficultyID
	if difficultyID == "" {
		difficultyID = catalog.DefaultDifficulty
	}
	difficulty, ok := catalog.Difficulties[difficultyID]
	if !ok {
		return domain.GameState{}, fmt.Errorf("%w: %s", ErrInvalidDifficulty, difficultyID)
	}
	seed := options.Seed
	if seed == 0 {
		var data [8]byte
		if _, err := rand.Read(data[:]); err != nil {
			return domain.GameState{}, fmt.Errorf("generate seed: %w", err)
		}
		seed = binary.LittleEndian.Uint64(data[:])
		if seed == 0 {
			seed = 1
		}
	}

	state := domain.GameState{
		SchemaVersion: catalog.Scenario.SchemaVersion,
		ScenarioID:    catalog.Scenario.ID,
		DifficultyID:  difficultyID,
		Phase:         domain.Decision,
		Resources:     difficulty.InitialResources,
		Environment: domain.EnvironmentState{
			Deforestation: difficulty.InitialDeforestation,
		},
		Sectors: make(map[domain.SectorID]domain.SectorState, len(catalog.Sectors)),
		Cards: domain.CardZones{
			Milestones: make(map[string]bool),
		},
		Events: domain.EventState{
			Cooldowns: make(map[string]int),
		},
		RNGState: seed,
	}

	for id, definition := range catalog.Sectors {
		state.Sectors[id] = domain.SectorState{Active: definition.InitiallyActive}
	}
	for _, id := range catalog.CardOrder {
		card := catalog.Cards[id]
		if card.StartingCard {
			sector := state.Sectors[card.Sector]
			sector.ActiveCards = append(sector.ActiveCards, card.ID)
			state.Sectors[card.Sector] = sector
			state.Cards.Milestones[card.ID] = true
			continue
		}
		state.Cards.Deck = append(state.Cards.Deck, card.ID)
	}
	shuffle(&state, state.Cards.Deck)
	drawCards(&state, catalog.Scenario.InitialHandSize)
	normalize(&state)
	return state, nil
}

func RulesFor(state domain.GameState, catalog domain.Catalog) (domain.ResolvedRules, error) {
	difficultyID := state.DifficultyID
	if difficultyID == "" {
		difficultyID = domain.LegacyDifficultyID
	}
	difficulty, ok := catalog.Difficulties[difficultyID]
	if !ok {
		return domain.ResolvedRules{}, fmt.Errorf("%w: %s", ErrInvalidDifficulty, difficultyID)
	}
	return domain.ResolvedRules{
		DifficultyID:                     difficultyID,
		MaxActiveEvents:                  difficulty.MaxActiveEvents,
		RandomEventProbabilityMultiplier: difficulty.RandomEventProbabilityMultiplier,
		BadGovernanceRoundInterval:       difficulty.BadGovernanceRoundInterval,
		SocialPressureLimit:              difficulty.SocialPressureLimit,
		TerritorialFailureLimit:          difficulty.TerritorialFailureLimit,
	}, nil
}

func Apply(state domain.GameState, command domain.Command, catalog domain.Catalog) (domain.Result, error) {
	if state.Defeat.GameOver || state.Victory.Completed || state.Phase == domain.Finished {
		return domain.Result{}, ErrGameAlreadyOver
	}
	if state.SchemaVersion != catalog.Scenario.SchemaVersion || state.ScenarioID != catalog.Scenario.ID {
		return domain.Result{}, fmt.Errorf("state is incompatible with catalog")
	}
	if _, err := RulesFor(state, catalog); err != nil {
		return domain.Result{}, err
	}
	if state.Phase == domain.DiscardRequired && command.Type != domain.DiscardCard {
		return domain.Result{}, ErrHandLimit
	}

	next, err := cloneState(state)
	if err != nil {
		return domain.Result{}, err
	}
	var emitted []domain.DomainEvent

	switch command.Type {
	case domain.PlayCard:
		err = playCard(&next, command.CardID, catalog, &emitted)
	case domain.ResolveEvent:
		err = resolveEvent(&next, command.EventID, catalog, &emitted)
	case domain.DiscardCard:
		err = discardCard(&next, command.CardID, catalog, &emitted)
	case domain.EndTurn:
		err = endTurn(&next, catalog, &emitted)
	default:
		err = ErrInvalidCommand
	}
	if err != nil {
		return domain.Result{}, err
	}

	if command.Type == domain.PlayCard || command.Type == domain.ResolveEvent {
		checkTippingPoints(&next, catalog, &emitted)
		evaluateOutcome(&next, catalog, &emitted)
	}
	normalize(&next)
	return domain.Result{State: next, Events: emitted}, nil
}

func cloneState(state domain.GameState) (domain.GameState, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return domain.GameState{}, fmt.Errorf("clone state: %w", err)
	}
	var cloned domain.GameState
	if err := json.Unmarshal(data, &cloned); err != nil {
		return domain.GameState{}, fmt.Errorf("clone state: %w", err)
	}
	return cloned, nil
}

func nextUint64(state *domain.GameState) uint64 {
	state.RNGState += 0x9e3779b97f4a7c15
	z := state.RNGState
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func nextFloat64(state *domain.GameState) float64 {
	return float64(nextUint64(state)>>11) / (1 << 53)
}

func shuffle(state *domain.GameState, cards []string) {
	for i := len(cards) - 1; i > 0; i-- {
		j := int(nextUint64(state) % uint64(i+1))
		cards[i], cards[j] = cards[j], cards[i]
	}
}

func drawCards(state *domain.GameState, amount int) {
	for range amount {
		if len(state.Cards.Deck) == 0 && len(state.Cards.Discard) > 0 {
			state.Cards.Deck = append(state.Cards.Deck, state.Cards.Discard...)
			state.Cards.Discard = nil
			shuffle(state, state.Cards.Deck)
		}
		if len(state.Cards.Deck) == 0 {
			return
		}
		last := len(state.Cards.Deck) - 1
		state.Cards.Hand = append(state.Cards.Hand, state.Cards.Deck[last])
		state.Cards.Deck = state.Cards.Deck[:last]
	}
}

func normalize(state *domain.GameState) {
	if state.Resources.Money < 0 {
		state.Resources.Money = 0
	}
	if state.Resources.People < 0 {
		state.Resources.People = 0
	}
	if state.Resources.Land < 0 {
		state.Resources.Land = 0
	}
	if state.Environment.Deforestation < 0 {
		state.Environment.Deforestation = 0
	}
	state.Environment.TemperatureLabel = temperatureLabel(state.Environment.Deforestation)
	if state.Defeat.GameOver || state.Victory.Completed {
		state.Phase = domain.Finished
	}
}

func temperatureLabel(deforestation int) string {
	switch {
	case deforestation >= 3000:
		return "3.0"
	case deforestation >= 2500:
		return "2.5"
	case deforestation >= 2000:
		return "2.0"
	case deforestation >= 1500:
		return "1.5"
	case deforestation >= 500:
		return "0.5"
	default:
		return "0.0"
	}
}
