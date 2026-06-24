package tui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hiteshsahu/squint/internal/model"
	"github.com/hiteshsahu/squint/internal/source"
)

// chromeHeight is the non-scrolling overhead around the viewport:
// header (1) + blank (1) + blank (1) + footer (1).
const chromeHeight = 4

type tickMsg time.Time

type snapMsg struct {
	snap *model.Snapshot
	err  error
}

// Model is the read-only L0 app: it polls a Source on an interval and renders
// the latest snapshot inside a scrollable viewport. No mutating commands exist
// yet — that's L1.
type Model struct {
	src      source.Source
	snap     *model.Snapshot
	err      error
	width    int
	height   int
	interval time.Duration
	vp       viewport.Model
	ready    bool
}

func New(src source.Source) Model {
	return Model{src: src, interval: 2 * time.Second}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(fetch(m.src), tick(m.interval))
}

func fetch(src source.Source) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		s, err := src.Snapshot(ctx)
		return snapMsg{snap: s, err: err}
	}
}

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "r":
			cmds = append(cmds, fetch(m.src))
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		vh := max(msg.Height-chromeHeight, 3)
		if !m.ready {
			m.vp = viewport.New(msg.Width, vh)
			m.ready = true
		} else {
			m.vp.Width = msg.Width
			m.vp.Height = vh
		}
		m.vp.SetContent(m.body())

	case tickMsg:
		cmds = append(cmds, fetch(m.src), tick(m.interval))

	case snapMsg:
		m.snap, m.err = msg.snap, msg.err
		if m.ready {
			y := m.vp.YOffset // keep scroll position across refreshes
			m.vp.SetContent(m.body())
			m.vp.SetYOffset(y)
		}
	}

	// Hand scroll keys / mouse wheel to the viewport.
	if m.ready {
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}
