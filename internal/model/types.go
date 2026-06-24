package model

import "time"

type JobState string

const (
	Running    JobState = "RUNNING"
	Pending    JobState = "PENDING"
	Completing JobState = "COMPLETING"
)

// Job is one entry in the Slurm queue.
type Job struct {
	ID        string
	Name      string
	User      string
	State     JobState
	Partition string
	Nodes     []string
	GPUReq    int
	Elapsed   time.Duration
	TimeLimit time.Duration
	Reason    string // raw Slurm pending reason code; "" while running
}

// GPU is a single device on a node, with live telemetry and its owning job.
type GPU struct {
	Index      int
	UtilPct    int
	MemUsedMB  int
	MemTotalMB int
	TempC      int
	PowerW     int
	JobID      string // owning job; "" when unallocated/free
	// HasTelemetry reports whether Util/Mem/Temp/Power are real readings. When
	// false we know allocation but not utilization, so we must NOT call an
	// allocated GPU a squatter just because Util reads 0.
	HasTelemetry bool
}

// Allocated reports whether a job currently holds this GPU.
func (g GPU) Allocated() bool { return g.JobID != "" }

// Squatting reports an allocated GPU doing essentially no work — the wasted
// capacity squint exists to surface. Requires real telemetry to assert.
func (g GPU) Squatting() bool { return g.Allocated() && g.HasTelemetry && g.UtilPct < 5 }

// Node is a compute host and the GPUs attached to it.
type Node struct {
	Name  string
	State string
	GPUs  []GPU
}

// Snapshot is a point-in-time view of the cluster.
type Snapshot struct {
	Jobs  []Job
	Nodes []Node
	Taken time.Time
}
