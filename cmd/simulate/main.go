package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/josephsae/colombia-ecosystems-engine/balance"
	"github.com/josephsae/colombia-ecosystems-engine/content"
)

func main() {
	games := flag.Int("games", 100, "partidas por dificultad")
	maxRounds := flag.Int("max-rounds", 80, "rondas maximas por partida")
	seed := flag.Uint64("seed", 1, "primera semilla")
	flag.Parse()
	catalog, err := content.LoadEmbedded()
	if err != nil {
		fatal(err)
	}
	report, err := balance.Run(catalog, balance.Config{GamesPerDifficulty: *games, MaxRounds: *maxRounds, FirstSeed: *seed})
	if err != nil {
		fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}
