package domain

const LegacyDifficultyID = "normal"

type ResourceID string

const (
	Money  ResourceID = "money"
	People ResourceID = "people"
	Land   ResourceID = "land"
)

type SectorID string

const (
	Industry   SectorID = "industry"
	Population SectorID = "population"
	Territory  SectorID = "territory"
	Ecosystems SectorID = "ecosystems"
	Global     SectorID = "global"
)

type CardType string

const (
	Action    CardType = "action"
	Structure CardType = "structure"
	Policy    CardType = "policy"
	Project   CardType = "project"
)

type EffectTrigger string

const (
	OnPlay             EffectTrigger = "on_play"
	OnSectorProduction EffectTrigger = "on_sector_production"
	Passive            EffectTrigger = "passive"
)

type EffectType string

const (
	GainResource                  EffectType = "gain_resource"
	LoseResource                  EffectType = "lose_resource"
	AddDeforestation              EffectType = "add_deforestation"
	RemoveDeforestation           EffectType = "remove_deforestation"
	ActivateSector                EffectType = "activate_sector"
	ModifySectorProduction        EffectType = "modify_sector_production"
	ModifySectorEnvironmentImpact EffectType = "modify_sector_environment_impact"
	BlockEvent                    EffectType = "block_event"
	BlockEventCategory            EffectType = "block_event_category"
	ModifyEventProbability        EffectType = "modify_event_probability"
	AddSocialPressure             EffectType = "add_social_pressure"
	RemoveSocialPressure          EffectType = "remove_social_pressure"
	CompleteProjectFlag           EffectType = "complete_project_flag"
	TriggerDefeat                 EffectType = "trigger_defeat"
)

type EventCategory string

const (
	TippingPoint  EventCategory = "tipping_point"
	RandomEvent   EventCategory = "random"
	Consequence   EventCategory = "consequence"
	BadGovernance EventCategory = "bad_governance"
	Terminal      EventCategory = "terminal"
)

type Phase string

const (
	Decision        Phase = "decision"
	DiscardRequired Phase = "discard_required"
	Finished        Phase = "finished"
)

type CommandType string

const (
	PlayCard     CommandType = "play_card"
	ResolveEvent CommandType = "resolve_event"
	DiscardCard  CommandType = "discard_card"
	EndTurn      CommandType = "end_turn"
)

type Resources struct {
	Money  int `json:"money"`
	People int `json:"people"`
	Land   int `json:"land"`
}

type Effect struct {
	Trigger  EffectTrigger `json:"trigger"`
	Type     EffectType    `json:"type"`
	Resource ResourceID    `json:"resource,omitempty"`
	Sector   SectorID      `json:"sector,omitempty"`
	Amount   int           `json:"amount,omitempty"`
	EventID  string        `json:"eventId,omitempty"`
	Category EventCategory `json:"category,omitempty"`
	Route    string        `json:"route,omitempty"`
	Flag     string        `json:"flag,omitempty"`
	Reason   string        `json:"reason,omitempty"`
}

type CardDefinition struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Type           CardType           `json:"type"`
	Sector         SectorID           `json:"sector"`
	StartingCard   bool               `json:"startingCard"`
	Cost           Resources          `json:"cost"`
	Requires       []string           `json:"requires"`
	Arrows         int                `json:"arrows"`
	Effects        []Effect           `json:"effects"`
	EventModifiers map[string]float64 `json:"eventModifiers"`
	VictoryTags    []string           `json:"victoryTags"`
	RulesText      string             `json:"rulesText"`
}

type SectorDefinition struct {
	ID                SectorID  `json:"id"`
	Name              string    `json:"name"`
	InitiallyActive   bool      `json:"initiallyActive"`
	CycleTarget       int       `json:"cycleTarget"`
	BaseAdvance       int       `json:"baseAdvance"`
	Production        Resources `json:"production"`
	EnvironmentImpact int       `json:"environmentImpact"`
}

type EventDefinition struct {
	ID                string        `json:"id"`
	Name              string        `json:"name"`
	Category          EventCategory `json:"category"`
	InitialRounds     int           `json:"initialRounds"`
	Solution          Resources     `json:"solution"`
	SolveEffects      []Effect      `json:"solveEffects"`
	ExpirationLoss    Resources     `json:"expirationLoss"`
	ExpirationEffects []Effect      `json:"expirationEffects"`
	CannotPayEffects  []Effect      `json:"cannotPayEffects"`
	Recurring         bool          `json:"recurring"`
	Terminal          bool          `json:"terminal"`
	BaseProbability   float64       `json:"baseProbability"`
	MinDeforestation  int           `json:"minDeforestation"`
	RequiresSector    SectorID      `json:"requiresSector,omitempty"`
	MinPeople         int           `json:"minPeople"`
	CooldownRounds    int           `json:"cooldownRounds"`
}

type TippingPointDefinition struct {
	Threshold int    `json:"threshold"`
	EventID   string `json:"eventId"`
}

type VictoryRoute struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	RequiredCards    []string `json:"requiredCards"`
	MinCards         int      `json:"minCards"`
	MaxDeforestation int      `json:"maxDeforestation"`
	MaxExclusive     bool     `json:"maxExclusive"`
	MinPeople        int      `json:"minPeople"`
	MinLand          int      `json:"minLand"`
	NoTerminalEvent  bool     `json:"noTerminalEvent"`
}

type VictoryModifiers struct {
	MinCards         int `json:"minCards"`
	MaxDeforestation int `json:"maxDeforestation"`
	MinPeople        int `json:"minPeople"`
	MinLand          int `json:"minLand"`
}

type DifficultyDefinition struct {
	ID                               string           `json:"id"`
	Name                             string           `json:"name"`
	InitialResources                 Resources        `json:"initialResources"`
	InitialDeforestation             int              `json:"initialDeforestation"`
	MaxActiveEvents                  int              `json:"maxActiveEvents"`
	RandomEventProbabilityMultiplier float64          `json:"randomEventProbabilityMultiplier"`
	BadGovernanceRoundInterval       int              `json:"badGovernanceRoundInterval"`
	SocialPressureLimit              int              `json:"socialPressureLimit"`
	TerritorialFailureLimit          int              `json:"territorialFailureLimit"`
	VictoryModifiers                 VictoryModifiers `json:"victoryModifiers"`
}

type ResolvedRules struct {
	DifficultyID                     string
	MaxActiveEvents                  int
	RandomEventProbabilityMultiplier float64
	BadGovernanceRoundInterval       int
	SocialPressureLimit              int
	TerritorialFailureLimit          int
}

type Scenario struct {
	ID                         string    `json:"id"`
	Name                       string    `json:"name"`
	SchemaVersion              int       `json:"schemaVersion"`
	InitialResources           Resources `json:"initialResources"`
	InitialDeforestation       int       `json:"initialDeforestation"`
	InitialHandSize            int       `json:"initialHandSize"`
	HandLimit                  int       `json:"handLimit"`
	DrawPerRound               int       `json:"drawPerRound"`
	MaxActiveEvents            int       `json:"maxActiveEvents"`
	BadGovernanceRoundInterval int       `json:"badGovernanceRoundInterval"`
	MaxEffectiveArrows         int       `json:"maxEffectiveArrows"`
}

type Catalog struct {
	Scenario          Scenario
	Cards             map[string]CardDefinition
	CardOrder         []string
	Sectors           map[SectorID]SectorDefinition
	Events            map[string]EventDefinition
	EventOrder        []string
	TippingPoints     []TippingPointDefinition
	VictoryRoutes     []VictoryRoute
	DefaultDifficulty string
	Difficulties      map[string]DifficultyDefinition
	DifficultyOrder   []string
}

type SectorState struct {
	Active        bool     `json:"active"`
	CycleProgress int      `json:"cycleProgress"`
	ActiveCards   []string `json:"activeCards"`
}

type EnvironmentState struct {
	Deforestation        int    `json:"deforestation"`
	TemperatureLabel     string `json:"temperatureLabel"`
	TippingPointsCrossed []int  `json:"tippingPointsCrossed"`
}

type CardZones struct {
	Hand       []string        `json:"hand"`
	Deck       []string        `json:"deck"`
	Discard    []string        `json:"discard"`
	Policies   []string        `json:"policies"`
	Projects   []string        `json:"projects"`
	Milestones map[string]bool `json:"milestones"`
}

type ActiveEvent struct {
	ID              string `json:"id"`
	RoundsRemaining int    `json:"roundsRemaining"`
}

type QueuedEvent struct {
	ID       string `json:"id"`
	Priority int    `json:"priority"`
}

type EventState struct {
	Active              []ActiveEvent  `json:"active"`
	Resolved            []string       `json:"resolved"`
	Queued              []QueuedEvent  `json:"queued"`
	Cooldowns           map[string]int `json:"cooldowns"`
	TerritorialFailures int            `json:"territorialFailures"`
}

type VictoryState struct {
	Completed bool   `json:"completed"`
	Route     string `json:"route,omitempty"`
}

type DefeatState struct {
	GameOver bool   `json:"gameOver"`
	Reason   string `json:"reason,omitempty"`
}

type GameState struct {
	SchemaVersion  int                      `json:"schemaVersion"`
	ScenarioID     string                   `json:"scenarioId"`
	DifficultyID   string                   `json:"difficultyId"`
	Round          int                      `json:"round"`
	Phase          Phase                    `json:"phase"`
	Resources      Resources                `json:"resources"`
	Environment    EnvironmentState         `json:"environment"`
	Sectors        map[SectorID]SectorState `json:"sectors"`
	Cards          CardZones                `json:"cards"`
	Events         EventState               `json:"events"`
	SocialPressure int                      `json:"socialPressure"`
	Victory        VictoryState             `json:"victory"`
	Defeat         DefeatState              `json:"defeat"`
	RNGState       uint64                   `json:"rngState"`
}

type Command struct {
	Type    CommandType `json:"type"`
	CardID  string      `json:"cardId,omitempty"`
	EventID string      `json:"eventId,omitempty"`
}

type DomainEvent struct {
	Type    string         `json:"type"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

type Result struct {
	State  GameState     `json:"state"`
	Events []DomainEvent `json:"events"`
}

type ActionAvailability struct {
	Allowed bool   `json:"allowed"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type AvailableActions struct {
	Cards      map[string]ActionAvailability `json:"cards"`
	Events     map[string]ActionAvailability `json:"events"`
	Discards   map[string]ActionAvailability `json:"discards"`
	CanEndTurn ActionAvailability            `json:"canEndTurn"`
}

type NewGameOptions struct {
	Seed         uint64
	DifficultyID string
}
