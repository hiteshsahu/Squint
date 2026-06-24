package source

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/hiteshsahu/squint/internal/model"
)

// Live reads a real Slurm cluster. Read-only by design: it only runs squeue and
// scontrol show node, plus optional GPU telemetry. Nothing it executes can
// mutate the cluster.
type Live struct {
	tel Telemetry
}

func NewLive() *Live {
	var tel Telemetry = noTelemetry{}
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		tel = newLocalSMI()
	}
	return &Live{tel: tel}
}

func (l *Live) Name() string { return "live" }

func (l *Live) Snapshot(ctx context.Context) (*model.Snapshot, error) {
	// squeue is the one mandatory call — it drives the jobs panel and the
	// pending-reason translator.
	jobsOut, err := run(ctx, "squeue", "--noheader", "--all", "-o", squeueFormat)
	if err != nil {
		return nil, fmt.Errorf("squeue failed: %w", err)
	}
	jobs := parseSqueue(jobsOut)

	// Node/GPU data is best-effort: if scontrol is unavailable, we still show
	// the queue rather than failing the whole snapshot.
	var nodes []model.Node
	if nodeOut, nerr := run(ctx, "scontrol", "show", "node", "--oneliner"); nerr == nil {
		nodes = parseScontrolNodes(nodeOut)
		l.tel.Enrich(ctx, nodes)
	}

	return &model.Snapshot{Jobs: jobs, Nodes: nodes, Taken: time.Now()}, nil
}

// run executes a read-only command and returns stdout, surfacing stderr on
// failure so errors are legible in the TUI.
func run(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}
