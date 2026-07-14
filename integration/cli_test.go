package integration_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIStartsAdvancesSavesAndLoads(t *testing.T) {
	binaryName := "amazonas"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binary := filepath.Join(t.TempDir(), binaryName)
	build := exec.Command("go", "build", "-o", binary, "./cmd/amazonas")
	build.Dir = ".."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	savePath := filepath.Join(t.TempDir(), "partida.json")
	created := exec.Command(binary, "new", "--seed", "42", "--difficulty", "easy")
	created.Stdin = strings.NewReader("end\nsave " + savePath + "\nquit\n")
	var createdOutput bytes.Buffer
	created.Stdout = &createdOutput
	created.Stderr = &createdOutput
	if err := created.Run(); err != nil {
		t.Fatalf("run new game: %v\n%s", err, createdOutput.String())
	}
	if !strings.Contains(createdOutput.String(), "Ronda 1") {
		t.Fatalf("CLI did not advance the round:\n%s", createdOutput.String())
	}
	if !strings.Contains(createdOutput.String(), "Dificultad: Facil") {
		t.Fatalf("CLI did not show difficulty:\n%s", createdOutput.String())
	}
	if _, err := os.Stat(savePath); err != nil {
		t.Fatalf("CLI did not create save: %v", err)
	}

	loaded := exec.Command(binary, "load", savePath)
	loaded.Stdin = strings.NewReader("status\nquit\n")
	var loadedOutput bytes.Buffer
	loaded.Stdout = &loadedOutput
	loaded.Stderr = &loadedOutput
	if err := loaded.Run(); err != nil {
		t.Fatalf("load saved game: %v\n%s", err, loadedOutput.String())
	}
	if !strings.Contains(loadedOutput.String(), "Ronda 1") || !strings.Contains(loadedOutput.String(), "Fase: discard_required") {
		t.Fatalf("CLI did not restore the game:\n%s", loadedOutput.String())
	}
}
