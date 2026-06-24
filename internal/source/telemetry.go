package source

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hiteshsahu/squint/internal/model"
)

// Telemetry enriches a node's GPUs with live util/mem/temp/power. It's optional:
// when nothing is available, GPUs render as alloc/free without utilization.
//
// The local nvidia-smi provider only knows the host squint runs on. Cluster-wide
// telemetry (DCGM-exporter via Prometheus) is the next provider to add here —
// same interface, different Enrich.
type Telemetry interface {
	Enrich(ctx context.Context, nodes []model.Node)
	Name() string
}

// noTelemetry is the null provider used on hosts without nvidia-smi.
type noTelemetry struct{}

func (noTelemetry) Enrich(context.Context, []model.Node) {}
func (noTelemetry) Name() string                         { return "none" }

// localSMI reads the local host's GPUs via nvidia-smi and applies the readings
// to the node whose name matches this host's short hostname.
type localSMI struct{ host string }

func newLocalSMI() *localSMI {
	h, _ := os.Hostname()
	return &localSMI{host: shortHost(h)}
}

func (s *localSMI) Name() string { return "nvidia-smi@" + s.host }

func (s *localSMI) Enrich(ctx context.Context, nodes []model.Node) {
	readings, err := s.read(ctx)
	if err != nil {
		return
	}
	for ni := range nodes {
		if !strings.EqualFold(shortHost(nodes[ni].Name), s.host) {
			continue
		}
		for gi := range nodes[ni].GPUs {
			if r, ok := readings[nodes[ni].GPUs[gi].Index]; ok {
				g := &nodes[ni].GPUs[gi]
				g.UtilPct, g.MemUsedMB, g.MemTotalMB, g.TempC, g.PowerW =
					r.util, r.memUsed, r.memTotal, r.temp, r.power
				g.HasTelemetry = true
			}
		}
	}
}

type smiReading struct{ util, memUsed, memTotal, temp, power int }

func (s *localSMI) read(ctx context.Context) (map[int]smiReading, error) {
	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=index,utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw",
		"--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	m := map[int]smiReading{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(line, ",")
		if len(f) < 6 {
			continue
		}
		m[atoi(f[0])] = smiReading{
			util:     atoi(f[1]),
			memUsed:  atoi(f[2]),
			memTotal: atoi(f[3]),
			temp:     atoi(f[4]),
			power:    atoi(f[5]), // power.draw is a float like 341.55; truncated
		}
	}
	return m, nil
}

// atoi parses the leading integer out of a possibly-noisy field ("85", "341.55",
// "[N/A]"), returning 0 on anything unparseable.
func atoi(s string) int {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

func shortHost(h string) string {
	if i := strings.IndexByte(h, '.'); i >= 0 {
		return h[:i]
	}
	return h
}
