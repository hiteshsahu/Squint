package source

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
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
		assignJobsToNodes(jobs, nodes)
		l.tel.Enrich(ctx, nodes)
	}

	return &model.Snapshot{Jobs: jobs, Nodes: nodes, Taken: time.Now()}, nil
}

func assignJobsToNodes(jobs []model.Job, nodes []model.Node) {
	nodeJobs := map[string][]string{}
	for _, node := range nodes {
		for _, job := range jobs {
			if job.State != model.Running {
				continue
			}
			if jobRunsOnNode(job, node.Name) {
				nodeJobs[node.Name] = append(nodeJobs[node.Name], job.ID)
			}
		}
	}

	for ni := range nodes {
		if len(nodeJobs[nodes[ni].Name]) != 1 {
			continue
		}
		jobID := nodeJobs[nodes[ni].Name][0]
		for gi := range nodes[ni].GPUs {
			if nodes[ni].GPUs[gi].Allocated() {
				nodes[ni].GPUs[gi].JobID = jobID
			}
		}
	}
}

func jobRunsOnNode(job model.Job, nodeName string) bool {
	for _, token := range job.Nodes {
		if compactHostlistContains(nodeName, token) {
			return true
		}
	}
	return false
}

func compactHostlistContains(nodeName, token string) bool {
	nodeName = strings.TrimSpace(nodeName)
	token = strings.TrimSpace(token)
	if nodeName == token {
		return true
	}
	open := strings.IndexByte(token, '[')
	close := strings.IndexByte(token, ']')
	if open < 0 || close < 0 || close <= open {
		return false
	}
	prefix := token[:open]
	suffix := token[close+1:]
	ranges := strings.Split(token[open+1:close], ",")
	for _, r := range ranges {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if i := strings.IndexByte(r, '-'); i >= 0 {
			lo, err1 := strconv.Atoi(r[:i])
			hi, err2 := strconv.Atoi(r[i+1:])
			if err1 != nil || err2 != nil || lo > hi {
				continue
			}
			width := len(r[:i])
			for v := lo; v <= hi; v++ {
				candidate := prefix + fmt.Sprintf("%0*d", width, v) + suffix
				if candidate == nodeName {
					return true
				}
			}
			continue
		}
		if prefix+r+suffix == nodeName {
			return true
		}
	}
	return false
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
