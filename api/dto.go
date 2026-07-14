package api

import (
	"sort"
	"time"

	"github.com/josephsae/colombia-ecosystems-engine/domain"
	"github.com/josephsae/colombia-ecosystems-engine/engine"
	"github.com/josephsae/colombia-ecosystems-engine/repository"
)

type SessionResponse struct {
	GuestID          string    `json:"guestId"`
	SessionID        string    `json:"sessionId"`
	AccessToken      string    `json:"accessToken"`
	TokenType        string    `json:"tokenType"`
	AccessExpiresAt  time.Time `json:"accessExpiresAt"`
	RefreshExpiresAt time.Time `json:"refreshExpiresAt"`
}

type CurrentSessionResponse struct {
	GuestID   string    `json:"guestId"`
	SessionID string    `json:"sessionId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type CreateGameRequest struct {
	Seed       uint64 `json:"seed,omitempty"`
	Difficulty string `json:"difficulty,omitempty"`
	TestPreset string `json:"testPreset,omitempty"`
}

type CommandRequest struct {
	Type            domain.CommandType `json:"type"`
	CardID          string             `json:"cardId,omitempty"`
	EventID         string             `json:"eventId,omitempty"`
	ExpectedVersion uint64             `json:"expectedVersion"`
}

type CardZonesView struct {
	Hand       []string        `json:"hand"`
	DeckCount  int             `json:"deckCount"`
	Discard    []string        `json:"discard"`
	Policies   []string        `json:"policies"`
	Projects   []string        `json:"projects"`
	Milestones map[string]bool `json:"milestones"`
}

type EventStateView struct {
	Active              []domain.ActiveEvent `json:"active"`
	Resolved            []string             `json:"resolved"`
	QueuedCount         int                  `json:"queuedCount"`
	TerritorialFailures int                  `json:"territorialFailures"`
}

type GameView struct {
	SchemaVersion  int                                    `json:"schemaVersion"`
	ScenarioID     string                                 `json:"scenarioId"`
	DifficultyID   string                                 `json:"difficultyId"`
	TestPresetID   string                                 `json:"testPresetId,omitempty"`
	Round          int                                    `json:"round"`
	Phase          domain.Phase                           `json:"phase"`
	Resources      domain.Resources                       `json:"resources"`
	Environment    domain.EnvironmentState                `json:"environment"`
	Sectors        map[domain.SectorID]domain.SectorState `json:"sectors"`
	Cards          CardZonesView                          `json:"cards"`
	Events         EventStateView                         `json:"events"`
	SocialPressure int                                    `json:"socialPressure"`
	Victory        domain.VictoryState                    `json:"victory"`
	Defeat         domain.DefeatState                     `json:"defeat"`
}

type GameResponse struct {
	ID               string                  `json:"id"`
	Version          uint64                  `json:"version"`
	CreatedAt        time.Time               `json:"createdAt"`
	UpdatedAt        time.Time               `json:"updatedAt"`
	State            GameView                `json:"state"`
	AvailableActions domain.AvailableActions `json:"availableActions"`
	DomainEvents     []domain.DomainEvent    `json:"domainEvents,omitempty"`
}

type GameSummary struct {
	ID           string              `json:"id"`
	Version      uint64              `json:"version"`
	Round        int                 `json:"round"`
	Phase        domain.Phase        `json:"phase"`
	DifficultyID string              `json:"difficultyId"`
	Victory      domain.VictoryState `json:"victory"`
	Defeat       domain.DefeatState  `json:"defeat"`
	CreatedAt    time.Time           `json:"createdAt"`
	UpdatedAt    time.Time           `json:"updatedAt"`
}

type ListGamesResponse struct {
	Games      []GameSummary `json:"games"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

type CatalogResponse struct {
	Scenario          domain.Scenario                 `json:"scenario"`
	Cards             []domain.CardDefinition         `json:"cards"`
	Sectors           []domain.SectorDefinition       `json:"sectors"`
	Events            []domain.EventDefinition        `json:"events"`
	TippingPoints     []domain.TippingPointDefinition `json:"tippingPoints"`
	VictoryRoutes     []domain.VictoryRoute           `json:"victoryRoutes"`
	DefaultDifficulty string                          `json:"defaultDifficulty"`
	Difficulties      []domain.DifficultyDefinition   `json:"difficulties"`
}

type ErrorBody struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

func gameResponse(game repository.StoredGame, catalog domain.Catalog, events []domain.DomainEvent) GameResponse {
	return GameResponse{
		ID: game.ID, Version: game.Version, CreatedAt: game.CreatedAt, UpdatedAt: game.UpdatedAt,
		State: publicState(game.State, domain.LegacyDifficultyID), AvailableActions: engine.Inspect(game.State, catalog), DomainEvents: events,
	}
}

func publicState(state domain.GameState, defaultDifficulty string) GameView {
	milestones := make(map[string]bool, len(state.Cards.Milestones))
	for key, value := range state.Cards.Milestones {
		milestones[key] = value
	}
	sectors := make(map[domain.SectorID]domain.SectorState, len(state.Sectors))
	for id, sector := range state.Sectors {
		sector.ActiveCards = append([]string(nil), sector.ActiveCards...)
		sectors[id] = sector
	}
	return GameView{
		SchemaVersion: state.SchemaVersion, ScenarioID: state.ScenarioID, DifficultyID: difficultyID(state, defaultDifficulty), TestPresetID: state.TestPresetID, Round: state.Round, Phase: state.Phase,
		Resources: state.Resources, Environment: state.Environment, Sectors: sectors,
		Cards: CardZonesView{
			Hand: append([]string(nil), state.Cards.Hand...), DeckCount: len(state.Cards.Deck),
			Discard: append([]string(nil), state.Cards.Discard...), Policies: append([]string(nil), state.Cards.Policies...),
			Projects: append([]string(nil), state.Cards.Projects...), Milestones: milestones,
		},
		Events: EventStateView{
			Active: append([]domain.ActiveEvent(nil), state.Events.Active...), Resolved: append([]string(nil), state.Events.Resolved...),
			QueuedCount: len(state.Events.Queued), TerritorialFailures: state.Events.TerritorialFailures,
		},
		SocialPressure: state.SocialPressure, Victory: state.Victory, Defeat: state.Defeat,
	}
}

func catalogResponse(catalog domain.Catalog) CatalogResponse {
	result := CatalogResponse{Scenario: catalog.Scenario, TippingPoints: catalog.TippingPoints, VictoryRoutes: catalog.VictoryRoutes, DefaultDifficulty: catalog.DefaultDifficulty}
	for _, id := range catalog.DifficultyOrder {
		result.Difficulties = append(result.Difficulties, catalog.Difficulties[id])
	}
	for _, id := range catalog.CardOrder {
		result.Cards = append(result.Cards, catalog.Cards[id])
	}
	for _, sector := range catalog.Sectors {
		result.Sectors = append(result.Sectors, sector)
	}
	sort.Slice(result.Sectors, func(i, j int) bool { return result.Sectors[i].ID < result.Sectors[j].ID })
	for _, id := range catalog.EventOrder {
		result.Events = append(result.Events, catalog.Events[id])
	}
	return result
}

func difficultyID(state domain.GameState, fallback string) string {
	if state.DifficultyID != "" {
		return state.DifficultyID
	}
	return fallback
}
