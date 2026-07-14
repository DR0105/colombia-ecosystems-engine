package balance

import (
	"fmt"
	"sort"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
	"github.com/josephsae/colombia-ecosystems-engine/engine"
)

type Config struct {
	GamesPerDifficulty int
	MaxRounds          int
	FirstSeed          uint64
}

type DifficultyReport struct {
	Difficulty    string         `json:"difficulty"`
	Games         int            `json:"games"`
	Victories     int            `json:"victories"`
	Defeats       int            `json:"defeats"`
	Timeouts      int            `json:"timeouts"`
	VictoryRate   float64        `json:"victoryRate"`
	AverageRounds float64        `json:"averageRounds"`
	VictoryRoutes map[string]int `json:"victoryRoutes"`
	DefeatReasons map[string]int `json:"defeatReasons"`
}

type Report struct {
	GamesPerDifficulty int                `json:"gamesPerDifficulty"`
	MaxRounds          int                `json:"maxRounds"`
	Results            []DifficultyReport `json:"results"`
}

func Run(catalog domain.Catalog, config Config) (Report, error) {
	if config.GamesPerDifficulty <= 0 || config.MaxRounds <= 0 {
		return Report{}, fmt.Errorf("games and max rounds must be positive")
	}
	if config.FirstSeed == 0 {
		config.FirstSeed = 1
	}
	report := Report{GamesPerDifficulty: config.GamesPerDifficulty, MaxRounds: config.MaxRounds}
	for _, difficultyID := range catalog.DifficultyOrder {
		result := DifficultyReport{
			Difficulty: difficultyID, Games: config.GamesPerDifficulty,
			VictoryRoutes: make(map[string]int), DefeatReasons: make(map[string]int),
		}
		totalRounds := 0
		for gameIndex := 0; gameIndex < config.GamesPerDifficulty; gameIndex++ {
			route := catalog.VictoryRoutes[gameIndex%len(catalog.VictoryRoutes)]
			seed := config.FirstSeed + uint64(gameIndex)
			state, err := engine.NewGame(catalog, domain.NewGameOptions{Seed: seed, DifficultyID: difficultyID})
			if err != nil {
				return Report{}, err
			}
			state = play(state, route, catalog, config.MaxRounds)
			totalRounds += state.Round
			switch {
			case state.Victory.Completed:
				result.Victories++
				result.VictoryRoutes[state.Victory.Route]++
			case state.Defeat.GameOver:
				result.Defeats++
				result.DefeatReasons[state.Defeat.Reason]++
			default:
				result.Timeouts++
			}
		}
		result.VictoryRate = float64(result.Victories) / float64(result.Games)
		result.AverageRounds = float64(totalRounds) / float64(result.Games)
		report.Results = append(report.Results, result)
	}
	return report, nil
}

func play(state domain.GameState, route domain.VictoryRoute, catalog domain.Catalog, maxRounds int) domain.GameState {
	needed := requiredCards(route, catalog)
	for state.Round < maxRounds && !state.Victory.Completed && !state.Defeat.GameOver {
		if state.Phase == domain.DiscardRequired {
			cardID := lowestScoringCard(state.Cards.Hand, route, needed, catalog, state)
			result, err := engine.Apply(state, domain.Command{Type: domain.DiscardCard, CardID: cardID}, catalog)
			if err != nil {
				return state
			}
			state = result.State
			continue
		}

		for {
			actions := engine.Inspect(state, catalog)
			eventID := resolvableEvent(state, actions)
			if eventID == "" {
				break
			}
			result, err := engine.Apply(state, domain.Command{Type: domain.ResolveEvent, EventID: eventID}, catalog)
			if err != nil {
				break
			}
			state = result.State
		}

		for {
			actions := engine.Inspect(state, catalog)
			cardID := bestPlayableCard(state, route, needed, catalog, actions)
			if cardID == "" {
				break
			}
			result, err := engine.Apply(state, domain.Command{Type: domain.PlayCard, CardID: cardID}, catalog)
			if err != nil {
				break
			}
			state = result.State
			if state.Victory.Completed || state.Defeat.GameOver {
				break
			}
		}
		if state.Victory.Completed || state.Defeat.GameOver {
			break
		}
		result, err := engine.Apply(state, domain.Command{Type: domain.EndTurn}, catalog)
		if err != nil {
			return state
		}
		state = result.State
	}
	return state
}

func requiredCards(route domain.VictoryRoute, catalog domain.Catalog) map[string]bool {
	needed := make(map[string]bool)
	var add func(string)
	add = func(id string) {
		if needed[id] {
			return
		}
		needed[id] = true
		for _, requirement := range catalog.Cards[id].Requires {
			add(requirement)
		}
	}
	for _, id := range route.RequiredCards {
		add(id)
	}
	return needed
}

func resolvableEvent(state domain.GameState, actions domain.AvailableActions) string {
	active := append([]domain.ActiveEvent(nil), state.Events.Active...)
	sort.Slice(active, func(i, j int) bool { return active[i].RoundsRemaining < active[j].RoundsRemaining })
	for _, event := range active {
		if actions.Events[event.ID].Allowed {
			return event.ID
		}
	}
	return ""
}

func bestPlayableCard(state domain.GameState, route domain.VictoryRoute, needed map[string]bool, catalog domain.Catalog, actions domain.AvailableActions) string {
	bestID := ""
	bestScore := 0
	for _, id := range state.Cards.Hand {
		if !actions.Cards[id].Allowed {
			continue
		}
		score := cardScore(id, route, needed, catalog, state)
		if score > bestScore || score == bestScore && (bestID == "" || id < bestID) {
			bestID, bestScore = id, score
		}
	}
	return bestID
}

func lowestScoringCard(hand []string, route domain.VictoryRoute, needed map[string]bool, catalog domain.Catalog, state domain.GameState) string {
	worstID := hand[0]
	worstScore := cardScore(worstID, route, needed, catalog, state)
	for _, id := range hand[1:] {
		score := cardScore(id, route, needed, catalog, state)
		if score < worstScore || score == worstScore && id < worstID {
			worstID, worstScore = id, score
		}
	}
	return worstID
}

func cardScore(id string, route domain.VictoryRoute, needed map[string]bool, catalog domain.Catalog, state domain.GameState) int {
	card := catalog.Cards[id]
	score := 5
	if needed[id] {
		score += 80
	}
	for _, requiredID := range route.RequiredCards {
		if id == requiredID {
			score += 80
			break
		}
	}
	if card.Type == domain.Structure {
		score += 15
	}
	for _, effect := range card.Effects {
		if effect.Type == domain.ActivateSector {
			score += 40
		}
		if effect.Type == domain.RemoveDeforestation || effect.Type == domain.GainResource || effect.Type == domain.RemoveSocialPressure {
			score += 10
		}
	}
	if id == "extractive_expansion" {
		if state.Resources.Money <= 1 && state.Environment.Deforestation < 2000 {
			return 30
		}
		return -100
	}
	return score
}
