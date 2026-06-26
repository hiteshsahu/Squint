package source

import (
	"reflect"
	"testing"
	"time"

	"github.com/hiteshsahu/squint/internal/model"
)

func TestParseSlurmDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"UNLIMITED", 0},
		{"INVALID", 0},
		{"N/A", 0},
		{"5", 5 * time.Second},
		{"30:00", 30 * time.Minute},
		{"1:23:45", time.Hour + 23*time.Minute + 45*time.Second},
		{"8:00:00", 8 * time.Hour},
		{"1-00:00:00", 24 * time.Hour},
		{"2-12:30:15", 2*24*time.Hour + 12*time.Hour + 30*time.Minute + 15*time.Second},
	}
	for _, c := range cases {
		if got := parseSlurmDuration(c.in); got != c.want {
			t.Errorf("parseSlurmDuration(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseGPUCount(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"(null)", 0},
		{"cpu=4,mem=16G", 0},
		{"gpu:4", 4},
		{"gpu:a100:8", 8},
		{"gpu:a100:4,gpu:v100:4", 8},
		{"gpu:a100:3(IDX:0-2)", 3},
	}
	for _, c := range cases {
		if got := parseGPUCount(c.in); got != c.want {
			t.Errorf("parseGPUCount(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseGPUCountWithFallback(t *testing.T) {
	cases := []struct {
		gres, tres string
		want       int
	}{
		{"gpu:0", "gpu:a100:16", 16},
		{"gpu:a100:4", "gpu:a100:8", 4},
		{"", "gpu:a100:2", 2},
	}

	for _, c := range cases {
		if got := parseGPUCountWithFallback(c.gres, c.tres); got != c.want {
			t.Errorf("parseGPUCountWithFallback(%q,%q) = %d, want %d", c.gres, c.tres, got, c.want)
		}
	}
}

func TestParseGresUsed(t *testing.T) {
	cases := []struct {
		in       string
		wantUsed int
		wantIdx  map[int]bool
	}{
		{"gpu:0", 0, map[int]bool{}},
		{"gpu:a100:3(IDX:0-2)", 3, map[int]bool{0: true, 1: true, 2: true}},
		{"gpu:a100:2(IDX:0,2)", 2, map[int]bool{0: true, 2: true}},
		{"gpu:a100:4(IDX:0-1,4-5)", 4, map[int]bool{0: true, 1: true, 4: true, 5: true}},
	}
	for _, c := range cases {
		used, idx := parseGresUsed(c.in)
		if used != c.wantUsed {
			t.Errorf("parseGresUsed(%q) used = %d, want %d", c.in, used, c.wantUsed)
		}
		if !reflect.DeepEqual(idx, c.wantIdx) {
			t.Errorf("parseGresUsed(%q) idx = %v, want %v", c.in, idx, c.wantIdx)
		}
	}
}

func TestRangePart(t *testing.T) {
	cases := []struct {
		in     string
		lo, hi int
		ok     bool
	}{
		{"0-2", 0, 2, true},
		{"5", 5, 5, true},
		{"", 0, 0, false},
		{"x", 0, 0, false},
		{"1-x", 0, 0, false},
	}
	for _, c := range cases {
		lo, hi, ok := rangePart(c.in)
		if lo != c.lo || hi != c.hi || ok != c.ok {
			t.Errorf("rangePart(%q) = (%d,%d,%v), want (%d,%d,%v)", c.in, lo, hi, ok, c.lo, c.hi, c.ok)
		}
	}
}

func TestCleanReason(t *testing.T) {
	cases := []struct{ in, want string }{
		{"None", ""},
		{"N/A", ""},
		{"Resources", "Resources"},
		{"(QOSMaxGRESPerUser)", "QOSMaxGRESPerUser"},
		{"  Priority  ", "Priority"},
	}
	for _, c := range cases {
		if got := cleanReason(c.in); got != c.want {
			t.Errorf("cleanReason(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMapState(t *testing.T) {
	cases := []struct {
		in   string
		want model.JobState
	}{
		{"RUNNING", model.Running},
		{"pending", model.Pending},
		{"COMPLETING", model.Completing},
		{"CONFIGURING", model.JobState("CONFIGURING")},
	}
	for _, c := range cases {
		if got := mapState(c.in); got != c.want {
			t.Errorf("mapState(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseSqueue(t *testing.T) {
	out := "1042|llama3-sft|alice|RUNNING|gpu|gpu-001|gpu:a100:4|gpu:a100:4|1:23:45|8:00:00|None\n" +
		"1050|big-pretrain|dave|PENDING|gpu||gpu:0|gpu:16|0:00|1-00:00:00|(QOSMaxGRESPerUser)\n"

	jobs := parseSqueue(out)
	if len(jobs) != 2 {
		t.Fatalf("parseSqueue returned %d jobs, want 2", len(jobs))
	}

	r := jobs[0]
	if r.ID != "1042" || r.User != "alice" || r.State != model.Running ||
		r.GPUReq != 4 || r.Elapsed != time.Hour+23*time.Minute+45*time.Second ||
		r.TimeLimit != 8*time.Hour || r.Reason != "" {
		t.Errorf("running job parsed wrong: %+v", r)
	}
	if !reflect.DeepEqual(r.Nodes, []string{"gpu-001"}) {
		t.Errorf("running job nodes = %v, want [gpu-001]", r.Nodes)
	}

	p := jobs[1]
	if p.State != model.Pending || p.GPUReq != 16 || p.TimeLimit != 24*time.Hour ||
		p.Reason != "QOSMaxGRESPerUser" || p.Nodes != nil {
		t.Errorf("pending job parsed wrong: %+v", p)
	}
}

func TestCompactHostlistContains(t *testing.T) {
	cases := []struct {
		node, token string
		want        bool
	}{
		{"gpu-001", "gpu-001", true},
		{"gpu-002", "gpu-[001-004]", true},
		{"gpu-010", "gpu-[001-004]", false},
		{"gpu-a", "gpu-[a-c]", false},
	}

	for _, c := range cases {
		if got := compactHostlistContains(c.node, c.token); got != c.want {
			t.Errorf("compactHostlistContains(%q,%q) = %v, want %v", c.node, c.token, got, c.want)
		}
	}
}

func TestAssignJobsToNodes(t *testing.T) {
	jobs := []model.Job{
		{ID: "100", State: model.Running, Nodes: []string{"gpu-[001-004]"}},
		{ID: "200", State: model.Running, Nodes: []string{"gpu-005"}},
	}
	nodes := []model.Node{
		{Name: "gpu-002", GPUs: make([]model.GPU, 4)},
		{Name: "gpu-005", GPUs: make([]model.GPU, 2)},
	}
	for i := range nodes[0].GPUs {
		nodes[0].GPUs[i].JobID = "alloc"
	}
	for i := range nodes[1].GPUs {
		nodes[1].GPUs[i].JobID = "alloc"
	}

	assignJobsToNodes(jobs, nodes)

	for _, g := range nodes[0].GPUs {
		if g.JobID != "100" {
			t.Errorf("expected gpu-002 GPU jobID to be 100, got %q", g.JobID)
		}
	}
	for _, g := range nodes[1].GPUs {
		if g.JobID != "200" {
			t.Errorf("expected gpu-005 GPU jobID to be 200, got %q", g.JobID)
		}
	}
}

func TestParseScontrolNodes(t *testing.T) {
	out := "NodeName=gpu-001 State=MIXED Gres=gpu:a100:8 GresUsed=gpu:a100:3(IDX:0-2) Partitions=gpu\n" +
		"NodeName=login-01 State=IDLE Gres=(null) GresUsed=gpu:0\n"

	nodes := parseScontrolNodes(out)
	if len(nodes) != 1 {
		t.Fatalf("parseScontrolNodes returned %d nodes, want 1 (login node has no GPUs)", len(nodes))
	}

	n := nodes[0]
	if n.Name != "gpu-001" || n.State != "MIXED" || len(n.GPUs) != 8 {
		t.Fatalf("node parsed wrong: name=%q state=%q gpus=%d", n.Name, n.State, len(n.GPUs))
	}
	for i, g := range n.GPUs {
		if g.Allocated() != (i <= 2) { // IDX:0-2
			t.Errorf("GPU %d allocated = %v, want %v", i, g.Allocated(), i <= 2)
		}
		if g.HasTelemetry {
			t.Errorf("GPU %d should have no telemetry from scontrol alone", i)
		}
	}
}
