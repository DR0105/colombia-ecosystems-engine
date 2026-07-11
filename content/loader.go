package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/josephsae/colombia-ecosystems-engine/assets"
	"github.com/josephsae/colombia-ecosystems-engine/domain"
)

type victoryFile struct {
	Routes []domain.VictoryRoute `json:"routes"`
}

type eventFile struct {
	Events        []domain.EventDefinition        `json:"events"`
	TippingPoints []domain.TippingPointDefinition `json:"tippingPoints"`
}

func LoadEmbedded() (domain.Catalog, error) {
	return LoadFS(assets.FS)
}

func LoadFS(source fs.FS) (domain.Catalog, error) {
	var scenario domain.Scenario
	var cards []domain.CardDefinition
	var sectors []domain.SectorDefinition
	var events eventFile
	var victories victoryFile

	if err := decode(source, "scenario.amazonas_mvp.json", &scenario); err != nil {
		return domain.Catalog{}, err
	}
	if err := decode(source, "cards.amazonas_mvp.json", &cards); err != nil {
		return domain.Catalog{}, err
	}
	if err := decode(source, "sectors.amazonas_mvp.json", &sectors); err != nil {
		return domain.Catalog{}, err
	}
	if err := decode(source, "events.amazonas_mvp.json", &events); err != nil {
		return domain.Catalog{}, err
	}
	if err := decode(source, "victory_routes.amazonas_mvp.json", &victories); err != nil {
		return domain.Catalog{}, err
	}

	catalog := domain.Catalog{
		Scenario:      scenario,
		Cards:         make(map[string]domain.CardDefinition, len(cards)),
		Sectors:       make(map[domain.SectorID]domain.SectorDefinition, len(sectors)),
		Events:        make(map[string]domain.EventDefinition, len(events.Events)),
		TippingPoints: events.TippingPoints,
		VictoryRoutes: victories.Routes,
	}
	for _, card := range cards {
		catalog.Cards[card.ID] = card
		catalog.CardOrder = append(catalog.CardOrder, card.ID)
	}
	for _, sector := range sectors {
		catalog.Sectors[sector.ID] = sector
	}
	for _, event := range events.Events {
		catalog.Events[event.ID] = event
		catalog.EventOrder = append(catalog.EventOrder, event.ID)
	}
	if err := Validate(catalog); err != nil {
		return domain.Catalog{}, err
	}
	return catalog, nil
}

func decode(source fs.FS, name string, target any) error {
	data, err := fs.ReadFile(source, name)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode %s: %w", name, err)
	}
	return nil
}

func Validate(c domain.Catalog) error {
	var problems []error
	if c.Scenario.ID == "" || c.Scenario.SchemaVersion < 1 {
		problems = append(problems, errors.New("invalid scenario metadata"))
	}
	if c.Scenario.InitialHandSize <= 0 || c.Scenario.HandLimit <= 0 || c.Scenario.InitialHandSize > c.Scenario.HandLimit {
		problems = append(problems, errors.New("invalid hand limits"))
	}
	if len(c.Cards) != 30 || len(c.CardOrder) != 30 {
		problems = append(problems, fmt.Errorf("expected 30 cards, got %d", len(c.Cards)))
	}
	if len(c.Sectors) != 4 {
		problems = append(problems, fmt.Errorf("expected 4 sectors, got %d", len(c.Sectors)))
	}
	starting := 0
	seenCards := map[string]bool{}
	for _, id := range c.CardOrder {
		if seenCards[id] {
			problems = append(problems, fmt.Errorf("duplicate card %q", id))
			continue
		}
		seenCards[id] = true
		card := c.Cards[id]
		if card.StartingCard {
			starting++
		}
		if card.ID == "" || card.Name == "" {
			problems = append(problems, fmt.Errorf("card %q has missing metadata", id))
		}
		if card.Sector != domain.Global {
			if _, ok := c.Sectors[card.Sector]; !ok {
				problems = append(problems, fmt.Errorf("card %q references unknown sector %q", id, card.Sector))
			}
		}
		if card.Cost.Money < 0 || card.Cost.People < 0 || card.Cost.Land < 0 || card.Arrows < 0 {
			problems = append(problems, fmt.Errorf("card %q has negative values", id))
		}
		for _, required := range card.Requires {
			if _, ok := c.Cards[required]; !ok {
				problems = append(problems, fmt.Errorf("card %q requires unknown card %q", id, required))
			}
		}
		for eventID, modifier := range card.EventModifiers {
			if _, ok := c.Events[eventID]; !ok {
				problems = append(problems, fmt.Errorf("card %q modifies unknown event %q", id, eventID))
			}
			if modifier < -1 || modifier > 1 {
				problems = append(problems, fmt.Errorf("card %q has invalid event modifier", id))
			}
		}
		for _, effect := range card.Effects {
			if err := validateEffect(effect, c); err != nil {
				problems = append(problems, fmt.Errorf("card %q: %w", id, err))
			}
		}
	}
	if starting != 1 {
		problems = append(problems, fmt.Errorf("expected one starting card, got %d", starting))
	}
	if err := validatePrerequisiteCycles(c); err != nil {
		problems = append(problems, err)
	}
	for _, sector := range c.Sectors {
		if sector.CycleTarget <= 0 || sector.BaseAdvance <= 0 {
			problems = append(problems, fmt.Errorf("sector %q has invalid cycle values", sector.ID))
		}
	}
	for _, event := range c.Events {
		if event.ID == "" || event.InitialRounds < 0 || event.BaseProbability < 0 || event.BaseProbability > 1 {
			problems = append(problems, fmt.Errorf("event %q has invalid values", event.ID))
		}
	}
	for _, point := range c.TippingPoints {
		if _, ok := c.Events[point.EventID]; !ok {
			problems = append(problems, fmt.Errorf("tipping point references unknown event %q", point.EventID))
		}
	}
	for _, route := range c.VictoryRoutes {
		if route.MinCards <= 0 || route.MinCards > len(route.RequiredCards) {
			problems = append(problems, fmt.Errorf("victory route %q has invalid card threshold", route.ID))
		}
		for _, id := range route.RequiredCards {
			if _, ok := c.Cards[id]; !ok {
				problems = append(problems, fmt.Errorf("victory route %q references unknown card %q", route.ID, id))
			}
		}
	}
	return errors.Join(problems...)
}

func validateEffect(effect domain.Effect, c domain.Catalog) error {
	if effect.Trigger != domain.OnPlay && effect.Trigger != domain.OnSectorProduction && effect.Trigger != domain.Passive {
		return fmt.Errorf("invalid trigger %q", effect.Trigger)
	}
	switch effect.Type {
	case domain.GainResource, domain.LoseResource:
		if effect.Resource != domain.Money && effect.Resource != domain.People && effect.Resource != domain.Land {
			return fmt.Errorf("invalid resource %q", effect.Resource)
		}
	case domain.ActivateSector, domain.ModifySectorProduction, domain.ModifySectorEnvironmentImpact:
		if _, ok := c.Sectors[effect.Sector]; !ok {
			return fmt.Errorf("unknown sector %q", effect.Sector)
		}
	case domain.BlockEvent, domain.ModifyEventProbability:
		if _, ok := c.Events[effect.EventID]; !ok {
			return fmt.Errorf("unknown event %q", effect.EventID)
		}
	case domain.AddDeforestation, domain.RemoveDeforestation, domain.AddSocialPressure,
		domain.RemoveSocialPressure, domain.BlockEventCategory, domain.CompleteProjectFlag, domain.TriggerDefeat:
		// Validated by the effect resolver or route configuration.
	default:
		return fmt.Errorf("unknown effect type %q", effect.Type)
	}
	return nil
}

func validatePrerequisiteCycles(c domain.Catalog) error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("card prerequisite cycle at %q", id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, required := range c.Cards[id].Requires {
			if err := visit(required); err != nil {
				return err
			}
		}
		delete(visiting, id)
		visited[id] = true
		return nil
	}
	ids := append([]string(nil), c.CardOrder...)
	sort.Strings(ids)
	for _, id := range ids {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}
