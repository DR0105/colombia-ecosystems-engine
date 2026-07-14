package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/josephsae/colombia-ecosystems-engine/content"
	"github.com/josephsae/colombia-ecosystems-engine/domain"
	"github.com/josephsae/colombia-ecosystems-engine/engine"
	"github.com/josephsae/colombia-ecosystems-engine/persistence"
)

func main() {
	catalog, err := content.LoadEmbedded()
	if err != nil {
		fatal(err)
	}
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var state domain.GameState
	switch os.Args[1] {
	case "new":
		flags := flag.NewFlagSet("new", flag.ExitOnError)
		seed := flags.Uint64("seed", 0, "semilla reproducible")
		difficulty := flags.String("difficulty", catalog.DefaultDifficulty, "dificultad: easy, normal o hard")
		_ = flags.Parse(os.Args[2:])
		state, err = engine.NewGame(catalog, domain.NewGameOptions{Seed: *seed, DifficultyID: *difficulty})
	case "load":
		flags := flag.NewFlagSet("load", flag.ExitOnError)
		_ = flags.Parse(os.Args[2:])
		if flags.NArg() != 1 {
			fatal(fmt.Errorf("uso: amazonas load <archivo>"))
		}
		state, err = persistence.Load(flags.Arg(0), catalog)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fatal(err)
	}
	runREPL(state, catalog)
}

func runREPL(state domain.GameState, catalog domain.Catalog) {
	fmt.Println("Motor Amazonas MVP")
	printStatus(state, catalog)
	reader := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("amazonas> ")
		if !reader.Scan() {
			fmt.Println()
			return
		}
		parts := strings.Fields(reader.Text())
		if len(parts) == 0 {
			continue
		}
		switch parts[0] {
		case "status":
			printStatus(state, catalog)
		case "hand":
			printHand(state, catalog)
		case "events":
			printEvents(state, catalog)
		case "play":
			state = applyCommand(state, catalog, parts, domain.PlayCard)
		case "resolve":
			state = applyCommand(state, catalog, parts, domain.ResolveEvent)
		case "discard":
			state = applyCommand(state, catalog, parts, domain.DiscardCard)
		case "end":
			result, err := engine.Apply(state, domain.Command{Type: domain.EndTurn}, catalog)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}
			state = result.State
			printDomainEvents(result.Events)
			printStatus(state, catalog)
		case "save":
			if len(parts) != 2 {
				fmt.Println("Uso: save <ruta>")
				continue
			}
			if err := persistence.Save(parts[1], state); err != nil {
				fmt.Println("Error:", err)
			} else {
				fmt.Println("Partida guardada en", parts[1])
			}
		case "help":
			printHelp()
		case "quit", "exit":
			return
		default:
			fmt.Println("Comando desconocido. Usa help.")
		}
	}
}

func applyCommand(state domain.GameState, catalog domain.Catalog, parts []string, commandType domain.CommandType) domain.GameState {
	if len(parts) != 2 {
		fmt.Println("El comando requiere un ID.")
		return state
	}
	command := domain.Command{Type: commandType}
	if commandType == domain.ResolveEvent {
		command.EventID = parts[1]
	} else {
		command.CardID = parts[1]
	}
	result, err := engine.Apply(state, command, catalog)
	if err != nil {
		fmt.Println("Error:", err)
		return state
	}
	printDomainEvents(result.Events)
	return result.State
}

func printStatus(state domain.GameState, catalog domain.Catalog) {
	difficulty := state.DifficultyID
	if difficulty == "" {
		difficulty = catalog.DefaultDifficulty
	}
	fmt.Printf("\nEscenario: %s | Dificultad: %s\n", catalog.Scenario.Name, catalog.Difficulties[difficulty].Name)
	fmt.Printf("Ronda %d | Fase: %s\n", state.Round, state.Phase)
	fmt.Printf("Recursos: dinero=%d personas=%d tierra=%d\n", state.Resources.Money, state.Resources.People, state.Resources.Land)
	fmt.Printf("Deforestacion: %d | Temperatura: %s | Presion social: %d\n", state.Environment.Deforestation, state.Environment.TemperatureLabel, state.SocialPressure)
	for _, id := range []domain.SectorID{domain.Industry, domain.Population, domain.Territory, domain.Ecosystems} {
		sector := state.Sectors[id]
		definition := catalog.Sectors[id]
		arrows := 0
		for _, cardID := range sector.ActiveCards {
			arrows += catalog.Cards[cardID].Arrows
		}
		fmt.Printf("- %s: activo=%t ciclo=%d/%d flechas=%d cartas=%d\n", definition.Name, sector.Active, sector.CycleProgress, definition.CycleTarget, arrows, len(sector.ActiveCards))
	}
	fmt.Printf("Mano=%d Mazo=%d Descarte=%d Eventos=%d\n", len(state.Cards.Hand), len(state.Cards.Deck), len(state.Cards.Discard), len(state.Events.Active))
	if state.Victory.Completed {
		fmt.Println("VICTORIA:", state.Victory.Route)
	}
	if state.Defeat.GameOver {
		fmt.Println("DERROTA:", state.Defeat.Reason)
	}
	fmt.Println()
}

func printHand(state domain.GameState, catalog domain.Catalog) {
	if len(state.Cards.Hand) == 0 {
		fmt.Println("La mano esta vacia.")
		return
	}
	for _, id := range state.Cards.Hand {
		card := catalog.Cards[id]
		fmt.Printf("%s | %s | %s | dinero=%d personas=%d tierra=%d\n  %s\n", card.ID, card.Name, card.Type, card.Cost.Money, card.Cost.People, card.Cost.Land, card.RulesText)
	}
}

func printEvents(state domain.GameState, catalog domain.Catalog) {
	if len(state.Events.Active) == 0 {
		fmt.Println("No hay eventos activos.")
		return
	}
	for _, active := range state.Events.Active {
		definition := catalog.Events[active.ID]
		fmt.Printf("%s | %s | rondas=%d | solucion=%d/%d/%d\n", definition.ID, definition.Name, active.RoundsRemaining, definition.Solution.Money, definition.Solution.People, definition.Solution.Land)
	}
}

func printDomainEvents(events []domain.DomainEvent) {
	for _, event := range events {
		fmt.Println("-", event.Message)
	}
}

func printHelp() {
	commands := []string{
		"status                 muestra el estado",
		"hand                   lista la mano",
		"play <card-id>         juega una carta",
		"events                 lista eventos activos",
		"resolve <event-id>     resuelve un evento",
		"discard <card-id>      descarta una carta",
		"end                    termina y resuelve la ronda",
		"save <ruta>            guarda la partida",
		"quit                   cierra la consola",
	}
	sort.Strings(commands)
	for _, command := range commands {
		fmt.Println(command)
	}
}

func usage() {
	fmt.Println("Uso:")
	fmt.Println("  amazonas new [--seed N] [--difficulty easy|normal|hard]")
	fmt.Println("  amazonas load <archivo>")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
