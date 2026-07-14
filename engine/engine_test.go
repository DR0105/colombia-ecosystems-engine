package engine

import (
	"errors"
	"reflect"
	"testing"

	"github.com/josephsae/colombia-ecosystems-engine/content"
	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

func testCatalog(t *testing.T) domain.Catalog {
	t.Helper()
	catalog, err := content.LoadEmbedded()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	return catalog
}

func testState(t *testing.T, catalog domain.Catalog) domain.GameState {
	t.Helper()
	state, err := NewGame(catalog, domain.NewGameOptions{Seed: 42, DifficultyID: "normal"})
	if err != nil {
		t.Fatalf("new game: %v", err)
	}
	return state
}

func TestNewGameDefaultsToEasy(t *testing.T) {
	catalog := testCatalog(t)
	state, err := NewGame(catalog, domain.NewGameOptions{Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	if state.DifficultyID != "easy" || state.Resources != (domain.Resources{Money: 3, People: 2, Land: 1}) || state.Environment.Deforestation != 250 {
		t.Fatalf("default state = difficulty %q resources %+v deforestation %d", state.DifficultyID, state.Resources, state.Environment.Deforestation)
	}
}

func TestNewGameIsDeterministic(t *testing.T) {
	catalog := testCatalog(t)
	first := testState(t, catalog)
	second := testState(t, catalog)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same seed produced different states")
	}
	if len(first.Cards.Hand) != 5 || len(first.Cards.Deck) != 24 {
		t.Fatalf("hand/deck = %d/%d, want 5/24", len(first.Cards.Hand), len(first.Cards.Deck))
	}
	if first.DifficultyID != "normal" {
		t.Fatalf("difficulty = %q, want normal", first.DifficultyID)
	}
	industry := first.Sectors[domain.Industry]
	if !industry.Active || !reflect.DeepEqual(industry.ActiveCards, []string{"livestock"}) {
		t.Fatalf("unexpected industry state: %+v", industry)
	}
}

func TestNewGameAppliesDifficultyAndRejectsUnknown(t *testing.T) {
	catalog := testCatalog(t)
	easy, err := NewGame(catalog, domain.NewGameOptions{Seed: 42, DifficultyID: "easy"})
	if err != nil {
		t.Fatal(err)
	}
	if easy.Resources != (domain.Resources{Money: 3, People: 2, Land: 1}) || easy.Environment.Deforestation != 250 {
		t.Fatalf("easy initial state = %+v/%d", easy.Resources, easy.Environment.Deforestation)
	}
	hard, err := NewGame(catalog, domain.NewGameOptions{Seed: 42, DifficultyID: "hard"})
	if err != nil {
		t.Fatal(err)
	}
	if hard.Resources != (domain.Resources{Money: 1, People: 1}) || hard.Environment.Deforestation != 500 {
		t.Fatalf("hard initial state = %+v/%d", hard.Resources, hard.Environment.Deforestation)
	}
	if _, err := NewGame(catalog, domain.NewGameOptions{DifficultyID: "unknown"}); !errors.Is(err, ErrInvalidDifficulty) {
		t.Fatalf("error = %v, want ErrInvalidDifficulty", err)
	}
}

func TestRulesAndEventProbabilityFollowDifficulty(t *testing.T) {
	catalog := testCatalog(t)
	definition := catalog.Events["forest_fires"]
	for _, test := range []struct {
		id          string
		maxEvents   int
		probability float64
	}{
		{id: "easy", maxEvents: 2, probability: 0.06},
		{id: "normal", maxEvents: 3, probability: 0.08},
		{id: "hard", maxEvents: 4, probability: 0.10},
	} {
		state, err := NewGame(catalog, domain.NewGameOptions{Seed: 42, DifficultyID: test.id})
		if err != nil {
			t.Fatal(err)
		}
		industry := state.Sectors[domain.Industry]
		industry.ActiveCards = nil
		state.Sectors[domain.Industry] = industry
		rules, err := RulesFor(state, catalog)
		if err != nil {
			t.Fatal(err)
		}
		if rules.MaxActiveEvents != test.maxEvents {
			t.Errorf("%s max events = %d", test.id, rules.MaxActiveEvents)
		}
		if got := effectiveEventProbability(state, definition, rules, catalog); got != test.probability {
			t.Errorf("%s probability = %v, want %v", test.id, got, test.probability)
		}
	}
}

func TestDifficultyControlsEventCapacityAndGovernmentInterval(t *testing.T) {
	catalog := testCatalog(t)
	easy, _ := NewGame(catalog, domain.NewGameOptions{Seed: 42, DifficultyID: "easy"})
	easy.Events.Active = []domain.ActiveEvent{{ID: "heat_wave"}, {ID: "famine"}}
	var emitted []domain.DomainEvent
	if !spawnEvent(&easy, "forest_fires", catalog, &emitted) || len(easy.Events.Active) != 2 || len(easy.Events.Queued) != 1 {
		t.Fatalf("easy event capacity not enforced: %+v", easy.Events)
	}

	normal, _ := NewGame(catalog, domain.NewGameOptions{Seed: 42, DifficultyID: "normal"})
	normal.Events.Active = []domain.ActiveEvent{{ID: "heat_wave"}, {ID: "famine"}}
	emitted = nil
	if !spawnEvent(&normal, "forest_fires", catalog, &emitted) || len(normal.Events.Active) != 3 || len(normal.Events.Queued) != 0 {
		t.Fatalf("normal event capacity not enforced: %+v", normal.Events)
	}

	hard, _ := NewGame(catalog, domain.NewGameOptions{Seed: 42, DifficultyID: "hard"})
	hard.Round = 12
	emitted = nil
	spawnBadGovernance(&hard, catalog, &emitted)
	if len(hard.Events.Active) != 1 || catalog.Events[hard.Events.Active[0].ID].Category != domain.BadGovernance {
		t.Fatalf("hard government interval did not spawn an event: %+v", hard.Events)
	}
}

func TestDifficultyChangesOutcomeThresholds(t *testing.T) {
	catalog := testCatalog(t)
	easy, _ := NewGame(catalog, domain.NewGameOptions{Seed: 42, DifficultyID: "easy"})
	easy.SocialPressure = 3
	easy.Cards.Projects = []string{"biodiversity_corridors", "community_agreement", "integral_reserve"}
	easy.Environment.Deforestation = 1400
	easy.Resources.Land = 2
	var emitted []domain.DomainEvent
	evaluateOutcome(&easy, catalog, &emitted)
	if easy.Defeat.GameOver || !easy.Victory.Completed || easy.Victory.Route != "restoration" {
		t.Fatalf("unexpected easy outcome: victory=%+v defeat=%+v", easy.Victory, easy.Defeat)
	}

	hard, _ := NewGame(catalog, domain.NewGameOptions{Seed: 42, DifficultyID: "hard"})
	hard.SocialPressure = 2
	emitted = nil
	evaluateOutcome(&hard, catalog, &emitted)
	if hard.Defeat.Reason != "social_collapse" {
		t.Fatalf("hard social defeat = %+v", hard.Defeat)
	}

	easy, _ = NewGame(catalog, domain.NewGameOptions{Seed: 42, DifficultyID: "easy"})
	easy.Events.TerritorialFailures = 2
	easy.Resources.People = 0
	easy.Events.Active = []domain.ActiveEvent{{ID: "famine", RoundsRemaining: 1}}
	emitted = nil
	evaluateOutcome(&easy, catalog, &emitted)
	if easy.Defeat.GameOver {
		t.Fatalf("easy game ended before its visible thresholds: %+v", easy.Defeat)
	}
}

func TestEndTurnRequiresDiscardWhenInitialHandDrawsAboveLimit(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	result, err := Apply(state, domain.Command{Type: domain.EndTurn}, catalog)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.State.Phase != domain.DiscardRequired {
		t.Fatalf("phase = %s, want %s", result.State.Phase, domain.DiscardRequired)
	}
	if len(result.State.Cards.Hand) != catalog.Scenario.HandLimit+1 {
		t.Fatalf("hand = %d, want %d", len(result.State.Cards.Hand), catalog.Scenario.HandLimit+1)
	}
}

func TestPlayActionCard(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	state.Cards.Hand = []string{"extractive_expansion"}
	result, err := Apply(state, domain.Command{Type: domain.PlayCard, CardID: "extractive_expansion"}, catalog)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.State.Resources.Money != 4 {
		t.Fatalf("money = %d, want 4", result.State.Resources.Money)
	}
	if result.State.Environment.Deforestation != 460 {
		t.Fatalf("deforestation = %d, want 460", result.State.Environment.Deforestation)
	}
	if !reflect.DeepEqual(result.State.Cards.Discard, []string{"extractive_expansion"}) {
		t.Fatalf("discard = %v", result.State.Cards.Discard)
	}
}

func TestCardPrerequisitesAreRequired(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	state.Cards.Hand = []string{"solar_industry"}
	state.Resources = domain.Resources{Money: 10, People: 10, Land: 10}
	_, err := Apply(state, domain.Command{Type: domain.PlayCard, CardID: "solar_industry"}, catalog)
	if !errors.Is(err, ErrRequirementsNotMet) {
		t.Fatalf("error = %v, want ErrRequirementsNotMet", err)
	}
}

func TestIndustryProducesUsingArrows(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	state.Cards.Deck = nil
	state.Cards.Discard = nil
	for i := 0; i < 2; i++ {
		result, err := Apply(state, domain.Command{Type: domain.EndTurn}, catalog)
		if err != nil {
			t.Fatalf("end turn %d: %v", i+1, err)
		}
		state = result.State
	}
	if state.Resources.Money != 3 {
		t.Fatalf("money = %d, want 3", state.Resources.Money)
	}
	if state.Environment.Deforestation != 360 {
		t.Fatalf("deforestation = %d, want 360", state.Environment.Deforestation)
	}
	if state.Sectors[domain.Industry].CycleProgress != 0 {
		t.Fatalf("industry progress = %d, want 0", state.Sectors[domain.Industry].CycleProgress)
	}
}

func TestEffectiveArrowsAreCapped(t *testing.T) {
	catalog := testCatalog(t)
	card := catalog.Cards["livestock"]
	card.Arrows = 10
	catalog.Cards["livestock"] = card
	state := testState(t, catalog)
	state.Cards.Deck = nil
	result, err := Apply(state, domain.Command{Type: domain.EndTurn}, catalog)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.State.Sectors[domain.Industry].CycleProgress != 1 {
		t.Fatalf("industry progress = %d, want 1", result.State.Sectors[domain.Industry].CycleProgress)
	}
	if result.State.Resources.Money != 3 {
		t.Fatalf("money = %d, want one production", result.State.Resources.Money)
	}
}

func TestPopulationFailureSpawnsFamine(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	industry := state.Sectors[domain.Industry]
	industry.Active = false
	state.Sectors[domain.Industry] = industry
	population := state.Sectors[domain.Population]
	population.CycleProgress = 4
	state.Sectors[domain.Population] = population
	state.Resources.Money = 0
	state.Cards.Deck = nil
	result, err := Apply(state, domain.Command{Type: domain.EndTurn}, catalog)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if activeEventIndex(result.State, "famine") < 0 {
		t.Fatalf("famine was not activated: %+v", result.State.Events)
	}
	if result.State.Resources.People != 1 {
		t.Fatalf("people = %d, want 1", result.State.Resources.People)
	}
}

func TestTippingPointOnlySpawnsOnce(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	state.Environment.Deforestation = 490
	state.Cards.Hand = []string{"extractive_expansion"}
	first, err := Apply(state, domain.Command{Type: domain.PlayCard, CardID: "extractive_expansion"}, catalog)
	if err != nil {
		t.Fatalf("first play: %v", err)
	}
	state = first.State
	state.Cards.Hand = append(state.Cards.Hand, "extractive_expansion")
	state.Resources.Money++
	second, err := Apply(state, domain.Command{Type: domain.PlayCard, CardID: "extractive_expansion"}, catalog)
	if err != nil {
		t.Fatalf("second play: %v", err)
	}
	count := 0
	for _, active := range second.State.Events.Active {
		if active.ID == "heat_wave" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("heat wave count = %d, want 1", count)
	}
}

func TestPartialEventPaymentAppliesPenalty(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	for id, sector := range state.Sectors {
		sector.Active = false
		state.Sectors[id] = sector
	}
	state.Resources = domain.Resources{Money: 1}
	state.Events.Active = []domain.ActiveEvent{{ID: "food_conflict", RoundsRemaining: 1}}
	state.Cards.Deck = nil
	result, err := Apply(state, domain.Command{Type: domain.EndTurn}, catalog)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.State.Resources.Money != 0 || result.State.SocialPressure != 1 {
		t.Fatalf("resources/pressure = %+v/%d", result.State.Resources, result.State.SocialPressure)
	}
	if result.State.Environment.Deforestation != 460 {
		t.Fatalf("deforestation = %d, want 460", result.State.Environment.Deforestation)
	}
}

func TestRestorationVictoryAndDefeatPriority(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	state.Cards.Projects = []string{"biodiversity_corridors", "community_agreement", "collective_titling", "integral_reserve"}
	state.Resources.Land = 3
	state.Environment.Deforestation = 1000
	var emitted []domain.DomainEvent
	evaluateOutcome(&state, catalog, &emitted)
	if !state.Victory.Completed || state.Victory.Route != "restoration" {
		t.Fatalf("unexpected victory: %+v", state.Victory)
	}

	state = testState(t, catalog)
	state.Cards.Projects = []string{"biodiversity_corridors", "community_agreement", "collective_titling", "integral_reserve"}
	state.Resources.Land = 3
	state.Environment.Deforestation = 3000
	emitted = nil
	evaluateOutcome(&state, catalog, &emitted)
	if !state.Defeat.GameOver || state.Victory.Completed {
		t.Fatalf("defeat did not take priority: defeat=%+v victory=%+v", state.Defeat, state.Victory)
	}
}

func TestSustainableTransitionVictory(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	state.Cards.Projects = []string{"energy_transition", "amazon_governance"}
	industry := state.Sectors[domain.Industry]
	industry.ActiveCards = append(industry.ActiveCards, "solar_industry", "circular_economy")
	state.Sectors[domain.Industry] = industry
	state.Resources.People = 5
	state.Environment.Deforestation = 2000
	var emitted []domain.DomainEvent
	evaluateOutcome(&state, catalog, &emitted)
	if !state.Victory.Completed || state.Victory.Route != "sustainable_transition" {
		t.Fatalf("unexpected victory: %+v", state.Victory)
	}
}

func TestSocialWellbeingVictoryCountsActionMilestone(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	state.Cards.Milestones["community_health"] = true
	state.Cards.Policies = []string{"peace_agreements"}
	state.Cards.Projects = []string{"amazon_governance"}
	population := state.Sectors[domain.Population]
	population.ActiveCards = append(population.ActiveCards, "community_guard")
	state.Sectors[domain.Population] = population
	state.Resources.People = 7
	state.Environment.Deforestation = 2000
	var emitted []domain.DomainEvent
	evaluateOutcome(&state, catalog, &emitted)
	if !state.Victory.Completed || state.Victory.Route != "social_wellbeing" {
		t.Fatalf("unexpected victory: %+v", state.Victory)
	}
}

func TestSocialAndTerritorialDefeats(t *testing.T) {
	catalog := testCatalog(t)

	social := testState(t, catalog)
	social.SocialPressure = 3
	var emitted []domain.DomainEvent
	evaluateOutcome(&social, catalog, &emitted)
	if social.Defeat.Reason != "social_collapse" {
		t.Fatalf("social defeat = %+v", social.Defeat)
	}

	territorial := testState(t, catalog)
	territorial.Events.TerritorialFailures = 2
	emitted = nil
	evaluateOutcome(&territorial, catalog, &emitted)
	if territorial.Defeat.Reason != "territorial_collapse" {
		t.Fatalf("territorial defeat = %+v", territorial.Defeat)
	}
}

func TestBadGovernanceBlockPreventsCorruption(t *testing.T) {
	catalog := testCatalog(t)
	state := testState(t, catalog)
	state.Round = 15
	state.Cards.Policies = append(state.Cards.Policies, "citizen_oversight")
	state.Cards.Deck = nil
	result, err := Apply(state, domain.Command{Type: domain.EndTurn}, catalog)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if eventExists(result.State, "corruption") {
		t.Fatal("corruption spawned despite citizen oversight")
	}
}
