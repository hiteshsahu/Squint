package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hiteshsahu/squint/internal/source"
	"github.com/hiteshsahu/squint/internal/tui"
)

func main() {
	// L0 is read-only. Default to the mock source so squint runs on a laptop
	// with no cluster in sight. The (stubbed) Live source will shell out to
	// squeue/sacct/scontrol + DCGM once L0 graduates off mock data.
	src := source.NewMock()

	// AltScreen for the full-screen TUI; mouse motion so the wheel scrolls the
	// viewport. (Selecting text in this mode needs Shift/Option, per terminal.)
	p := tea.NewProgram(tui.New(src), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "squint:", err)
		os.Exit(1)
	}
}
