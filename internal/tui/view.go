package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/hiteshsahu/squint/internal/model"
	"github.com/hiteshsahu/squint/internal/source"
)

// On-brand palette (matches the hiteshsahu.com accents).
var (
	gold  = lipgloss.Color("#f4b53f")
	teal  = lipgloss.Color("#2dd4bf")
	coral = lipgloss.Color("#fb7a6b")
	dim   = lipgloss.Color("#6b7280")
	fg    = lipgloss.Color("#e5e7eb")
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#0b0b0b")).Background(teal).Padding(0, 1)
	paneStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(dim).Padding(0, 1)
	headStyle  = lipgloss.NewStyle().Bold(true).Foreground(gold)
	dimStyle   = lipgloss.NewStyle().Foreground(dim)
	fgStyle    = lipgloss.NewStyle().Foreground(fg)
)

// paneFrame is the horizontal overhead a pane adds: rounded border (1+1) plus
// Padding(0,1) (1+1).
const paneFrame = 4

func (m Model) View() string {
	if !m.ready {
		return "\n  connecting to " + m.src.Name() + "…\n"
	}
	header := titleStyle.Render("squint") + "  " +
		dimStyle.Render("GPU-aware Slurm monitor · read-only")
	foot := dimStyle.Render(m.footer())
	return lipgloss.JoinVertical(lipgloss.Left, header, "", m.vp.View(), "", foot)
}

func (m Model) footer() string {
	src, upd := "—", ""
	if m.snap != nil {
		src, upd = m.src.Name(), m.snap.Taken.Format("15:04:05")
	}
	scroll := ""
	if m.vp.TotalLineCount() > m.vp.Height {
		scroll = fmt.Sprintf("   %3.0f%% · ↑↓/wheel scroll", m.vp.ScrollPercent()*100)
	}
	return fmt.Sprintf("source:%s   updated %s   [r] refresh   [q] quit%s", src, upd, scroll)
}

// body is the scrollable content: the two panes, laid out for the current width.
func (m Model) body() string {
	if m.err != nil {
		return fmt.Sprintf("squint error: %v", m.err)
	}
	if m.snap == nil {
		return "connecting to " + m.src.Name() + "…"
	}

	totalW := m.width
	if totalW <= 0 {
		totalW = 100
	}

	heatmap := paneStyle.Render(m.renderHeatmap())
	heatmapW := lipgloss.Width(heatmap)

	const gap = 2
	const minJobsTotal = 50 // narrowest the jobs pane stays readable at

	if totalW >= heatmapW+gap+minJobsTotal {
		// Side by side: give the jobs pane whatever's left of the row.
		jobsTotal := totalW - gap - heatmapW
		jobs := paneStyle.Render(m.renderJobs(jobsTotal - paneFrame))
		return lipgloss.JoinHorizontal(lipgloss.Top, jobs, strings.Repeat(" ", gap), heatmap)
	}
	// Too narrow: stack, heatmap (the hero) on top, jobs full width below.
	jobs := paneStyle.Render(m.renderJobs(totalW - paneFrame))
	return lipgloss.JoinVertical(lipgloss.Left, heatmap, "", jobs)
}

// renderHeatmap draws each node as a row of GPU cells and shames any GPU that's
// allocated but idle. Naturally ~56 cols wide (8 cells), so it drives layout.
func (m Model) renderHeatmap() string {
	var b strings.Builder
	b.WriteString(headStyle.Render("NODES & GPUs") + "\n\n")

	squat := 0
	for _, n := range m.snap.Nodes {
		cells := make([]string, len(n.GPUs))
		for i, g := range n.GPUs {
			cells[i] = gpuCell(g)
			if g.Squatting() {
				squat++
			}
		}
		label := lipgloss.NewStyle().Width(10).Foreground(fg).Render(n.Name) +
			dimStyle.Render(n.State)
		row := lipgloss.JoinHorizontal(lipgloss.Top, cells...)
		b.WriteString(label + "\n" + row + "\n\n")
	}

	if squat > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(coral).Bold(true).
			Render(fmt.Sprintf("⚠  %d GPU(s) allocated but idle — someone's squatting", squat)))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(teal).
			Render("✓ no idle-but-allocated GPUs"))
	}
	return b.String()
}

// gpuCell renders one device, colored by what it's doing.
func gpuCell(g model.GPU) string {
	box := lipgloss.NewStyle().Width(6).Align(lipgloss.Center).MarginRight(1)

	var col lipgloss.Color
	var bot string
	switch {
	case !g.Allocated():
		col, bot = dim, "free"
	case g.Squatting():
		col, bot = coral, "IDLE"
	case g.UtilPct >= 70:
		col, bot = gold, fmt.Sprintf("%d%%", g.UtilPct)
	default:
		col, bot = teal, fmt.Sprintf("%d%%", g.UtilPct)
	}

	style := box.Foreground(col)
	if g.Squatting() {
		style = style.Bold(true)
	}
	return style.Render(fmt.Sprintf("G%d\n%s", g.Index, bot))
}

// renderJobs lists running jobs, then pending jobs with a plain-English reason
// wrapped to the given inner content width.
func (m Model) renderJobs(width int) string {
	if width < 24 {
		width = 24
	}
	var b strings.Builder
	b.WriteString(headStyle.Render("JOBS") + "\n\n")

	b.WriteString(dimStyle.Render("running") + "\n")
	for _, j := range m.snap.Jobs {
		if j.State != model.Running {
			continue
		}
		b.WriteString(fgStyle.Render(fmt.Sprintf(
			"%-6s %-12s %-7s %2dxGPU  %s",
			j.ID, trunc(j.Name, 12), j.User, j.GPUReq, fmtDur(j.Elapsed),
		)) + "\n")
	}

	b.WriteString("\n" + dimStyle.Render("pending  ") + headStyle.Render("— why?") + "\n")
	for _, j := range m.snap.Jobs {
		if j.State != model.Pending {
			continue
		}
		ex := source.Explain(j.Reason)
		b.WriteString(lipgloss.NewStyle().Foreground(gold).Render(fmt.Sprintf(
			"%-6s %-12s %-7s %2dxGPU", j.ID, trunc(j.Name, 12), j.User, j.GPUReq,
		)) + "\n")
		b.WriteString(fgStyle.Render(indentWrap(ex.Plain, 3, width)) + "\n")
		if ex.Suggestion != "" {
			b.WriteString(dimStyle.Render(indentWrap("→ "+ex.Suggestion, 3, width)) + "\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// indentWrap word-wraps s to fit within width columns, indenting every line by
// indent spaces.
func indentWrap(s string, indent, width int) string {
	avail := width - indent
	if avail < 12 {
		avail = 12
	}
	wrapped := lipgloss.NewStyle().Width(avail).Render(s)
	pad := strings.Repeat(" ", indent)
	lines := strings.Split(wrapped, "\n")
	for i := range lines {
		lines[i] = pad + strings.TrimRight(lines[i], " ")
	}
	return strings.Join(lines, "\n")
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func fmtDur(d time.Duration) string {
	d = d.Round(time.Minute)
	return fmt.Sprintf("%dh%02dm", d/time.Hour, (d%time.Hour)/time.Minute)
}
