package source

import (
	"context"
	"math/rand"
	"time"

	"github.com/hiteshsahu/squint/internal/model"
)

// Source yields point-in-time cluster snapshots. Mock runs anywhere; Live
// reads a real Slurm cluster via squeue/scontrol + GPU telemetry.
type Source interface {
	Snapshot(ctx context.Context) (*model.Snapshot, error)
	Name() string
}

// ---------------------------------------------------------------------------
// Mock: a deterministic, slightly-jittering cluster so the TUI runs and "feels
// live" without a Slurm controller. Intentionally seeded with one squatter so
// the idle-GPU shaming has something to shame.
// ---------------------------------------------------------------------------

type Mock struct {
	r     *rand.Rand
	nodes []model.Node
	jobs  []model.Job
	start time.Time
}

func NewMock() *Mock {
	m := &Mock{r: rand.New(rand.NewSource(7)), start: time.Now()}
	m.seed()
	return m
}

func (m *Mock) Name() string { return "mock" }

func (m *Mock) seed() {
	mk := func(name, state string, n int) model.Node {
		gpus := make([]model.GPU, n)
		for i := range gpus {
			gpus[i] = model.GPU{Index: i, MemTotalMB: 81920} // A100 80GB
		}
		return model.Node{Name: name, State: state, GPUs: gpus}
	}
	n1 := mk("gpu-001", "MIXED", 8)
	n2 := mk("gpu-002", "ALLOCATED", 8)
	n3 := mk("gpu-003", "MIXED", 8)

	for i := 0; i <= 3; i++ {
		n1.GPUs[i].JobID = "1042" // alice running hot on 0-3
	}
	for i := range n2.GPUs {
		n2.GPUs[i].JobID = "1043" // bob holds all 8 but squats — the shame shot
	}
	n3.GPUs[0].JobID = "1044" // carol on 0-1
	n3.GPUs[1].JobID = "1044"

	m.nodes = []model.Node{n1, n2, n3}

	m.jobs = []model.Job{
		{ID: "1042", Name: "llama3-sft", User: "alice", State: model.Running, Partition: "gpu", Nodes: []string{"gpu-001"}, GPUReq: 4, TimeLimit: 8 * time.Hour},
		{ID: "1043", Name: "dpo-run", User: "bob", State: model.Running, Partition: "gpu", Nodes: []string{"gpu-002"}, GPUReq: 8, TimeLimit: 12 * time.Hour},
		{ID: "1044", Name: "eval-sweep", User: "carol", State: model.Running, Partition: "gpu", Nodes: []string{"gpu-003"}, GPUReq: 2, TimeLimit: 2 * time.Hour},
		{ID: "1050", Name: "big-pretrain", User: "dave", State: model.Pending, Partition: "gpu", GPUReq: 16, TimeLimit: 24 * time.Hour, Reason: "QOSMaxGRESPerUser"},
		{ID: "1051", Name: "hp-search", User: "erin", State: model.Pending, Partition: "gpu", GPUReq: 4, TimeLimit: 6 * time.Hour, Reason: "Priority"},
		{ID: "1052", Name: "infer-bench", User: "frank", State: model.Pending, Partition: "gpu", GPUReq: 8, TimeLimit: 1 * time.Hour, Reason: "Resources"},
		{ID: "1053", Name: "finetune-xl", User: "grace", State: model.Pending, Partition: "gpu-hi", GPUReq: 2, TimeLimit: 4 * time.Hour, Reason: "ReqNodeNotAvail"},
		{ID: "1054", Name: "data-prep", User: "heidi", State: model.Pending, Partition: "cpu", GPUReq: 0, TimeLimit: 30 * time.Minute, Reason: "Dependency"},
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m *Mock) Snapshot(ctx context.Context) (*model.Snapshot, error) {
	elapsed := time.Since(m.start)

	for ni := range m.nodes {
		for gi := range m.nodes[ni].GPUs {
			g := &m.nodes[ni].GPUs[gi]
			g.HasTelemetry = true // the mock always has telemetry
			switch {
			case g.JobID == "": // free
				g.UtilPct, g.MemUsedMB, g.TempC, g.PowerW = 0, 0, 31, 58
			case g.JobID == "1043": // squatter: allocated, ~idle
				g.UtilPct = clamp(2+m.r.Intn(3), 0, 100)
				g.MemUsedMB = 1200
				g.TempC = 34 + m.r.Intn(2)
				g.PowerW = 70 + m.r.Intn(8)
			default: // working hard
				g.UtilPct = clamp(88+m.r.Intn(12)-6, 0, 100)
				g.MemUsedMB = clamp(64000+m.r.Intn(8000), 0, g.MemTotalMB)
				g.TempC = 62 + m.r.Intn(8)
				g.PowerW = 340 + m.r.Intn(60)
			}
		}
	}

	jobs := make([]model.Job, len(m.jobs))
	copy(jobs, m.jobs)
	for i := range jobs {
		if jobs[i].State == model.Running {
			jobs[i].Elapsed = elapsed + time.Duration(i)*7*time.Minute
		}
	}
	nodes := make([]model.Node, len(m.nodes))
	copy(nodes, m.nodes)

	return &model.Snapshot{Jobs: jobs, Nodes: nodes, Taken: time.Now()}, nil
}
