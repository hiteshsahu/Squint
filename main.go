package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hiteshsahu/squint/internal/source"
	"github.com/hiteshsahu/squint/internal/tui"
)

func main() {

	// --live
	live := flag.Bool("live", false, "read a real Slurm cluster (squeue + scontrol + nvidia-smi) instead of mock data")
	flag.Parse()

	// L0 is read-only either way. Mock runs anywhere; --live shells out to the
	// cluster. GPU telemetry needs nvidia-smi on the host squint runs on.
	var src source.Source = source.NewMock()
	if *live {
		src = source.NewLive()
	}

	// AltScreen for the full-screen TUI; mouse motion so the wheel scrolls the
	// viewport. (Selecting text in this mode needs Shift/Option, per terminal.)
	program := tea.NewProgram(
		tui.New(src),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		_, err := fmt.Fprintln(os.Stderr, "squint:", err)
		if err != nil {
			return
		}
		os.Exit(1)
	}
}
