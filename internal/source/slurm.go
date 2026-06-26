package source

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hiteshsahu/squint/internal/model"
)

// squeueFormat is pipe-delimited so we can split cleanly. Fields:
// id|name|user|state|partition|nodelist|gres|tres-per-node|elapsed|timelimit|reason
//
// NOTE: %b is the GRES field and %B is the TRES-per-node field. Some Slurm
// installations report GPU request counts in TRES instead of GRES, so we parse
// both and fall back from GRES to TRES when needed.
const squeueFormat = "%i|%j|%u|%T|%P|%N|%b|%B|%M|%l|%r"

func parseSqueue(out string) []model.Job {
	var jobs []model.Job
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, "|")
		if len(f) < 11 {
			continue
		}
		jobs = append(jobs, model.Job{
			ID:        strings.TrimSpace(f[0]),
			Name:      strings.TrimSpace(f[1]),
			User:      strings.TrimSpace(f[2]),
			State:     mapState(f[3]),
			Partition: strings.TrimSpace(f[4]),
			Nodes:     splitNodes(f[5]),
			GPUReq:    parseGPUCountWithFallback(strings.TrimSpace(f[6]), strings.TrimSpace(f[7])),
			Elapsed:   parseSlurmDuration(f[8]),
			TimeLimit: parseSlurmDuration(f[9]),
			Reason:    cleanReason(f[10]),
		})
	}
	return jobs
}

func parseGPUCountWithFallback(gres, tres string) int {
	if cnt := parseGPUCount(gres); cnt > 0 {
		return cnt
	}
	return parseGPUCount(tres)
}

func mapState(s string) model.JobState {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "RUNNING":
		return model.Running
	case "PENDING":
		return model.Pending
	case "COMPLETING":
		return model.Completing
	default:
		return model.JobState(strings.ToUpper(strings.TrimSpace(s)))
	}
}

func cleanReason(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "(")
	s = strings.TrimSuffix(s, ")")
	if s == "None" || s == "N/A" {
		return ""
	}
	return s
}

// splitNodes keeps the compact hostlist as-is for v0 (e.g. "gpu-[001-003]").
// Full hostlist expansion is future work.
func splitNodes(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "(null)" {
		return nil
	}
	return []string{s}
}

// gpuRe matches gpu[:type]:N anywhere in a GRES/TRES string.
var gpuRe = regexp.MustCompile(`(?i)gpu(?::[^:,()]+)?:(\d+)`)

func parseGPUCount(s string) int {
	total := 0
	for _, m := range gpuRe.FindAllStringSubmatch(s, -1) {
		n, _ := strconv.Atoi(m[1])
		total += n
	}
	return total
}

// parseSlurmDuration parses [DD-]HH:MM:SS / HH:MM:SS / MM:SS. Returns 0 for
// UNLIMITED / INVALID / N/A / empty.
func parseSlurmDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	switch s {
	case "", "UNLIMITED", "INVALID", "N/A", "NOT_SET":
		return 0
	}
	days := 0
	if i := strings.IndexByte(s, '-'); i >= 0 {
		days, _ = strconv.Atoi(s[:i])
		s = s[i+1:]
	}
	parts := strings.Split(s, ":")
	var h, m, sec int
	switch len(parts) {
	case 3:
		h, _ = strconv.Atoi(parts[0])
		m, _ = strconv.Atoi(parts[1])
		sec, _ = strconv.Atoi(parts[2])
	case 2:
		m, _ = strconv.Atoi(parts[0])
		sec, _ = strconv.Atoi(parts[1])
	case 1:
		sec, _ = strconv.Atoi(parts[0])
	default:
		return 0
	}
	return time.Duration(days)*24*time.Hour +
		time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(sec)*time.Second
}

// ---------------------------------------------------------------------------
// scontrol show node --oneliner
// ---------------------------------------------------------------------------

var idxRe = regexp.MustCompile(`IDX:([0-9,\-]+)`)

func parseScontrolNodes(out string) []model.Node {
	var nodes []model.Node
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if !strings.HasPrefix(line, "NodeName=") {
			continue
		}
		kv := parseKV(line)
		name := kv["NodeName"]
		total := parseGPUCount(kv["Gres"])
		if total == 0 {
			total = parseGPUCount(kv["GresUsed"])
		}
		if name == "" || total == 0 {
			continue // skip GPU-less nodes; this is a GPU monitor
		}
		used, idx := parseGresUsed(kv["GresUsed"])

		gpus := make([]model.GPU, total)
		for i := range gpus {
			gpus[i] = model.GPU{Index: i}
			switch {
			case idx[i]:
				gpus[i].JobID = "alloc" // placeholder; job attribution is future work
			case len(idx) == 0 && i < used:
				gpus[i].JobID = "alloc" // no IDX map available — fall back to count
			}
		}
		nodes = append(nodes, model.Node{Name: name, State: kv["State"], GPUs: gpus})
	}
	return nodes
}

func parseKV(line string) map[string]string {
	out := map[string]string{}
	for _, tok := range strings.Fields(line) {
		if i := strings.IndexByte(tok, '='); i > 0 {
			out[tok[:i]] = tok[i+1:]
		}
	}
	return out
}

// parseGresUsed returns the used GPU count and the set of allocated indices
// from e.g. "gpu:a100:3(IDX:0-2)".
func parseGresUsed(s string) (int, map[int]bool) {
	used := parseGPUCount(s)
	idx := map[int]bool{}
	for _, m := range idxRe.FindAllStringSubmatch(s, -1) {
		for _, part := range strings.Split(m[1], ",") {
			if lo, hi, ok := rangePart(part); ok {
				for i := lo; i <= hi; i++ {
					idx[i] = true
				}
			}
		}
	}
	return used, idx
}

func rangePart(p string) (lo, hi int, ok bool) {
	p = strings.TrimSpace(p)
	if p == "" {
		return 0, 0, false
	}
	if i := strings.IndexByte(p, '-'); i >= 0 {
		a, e1 := strconv.Atoi(p[:i])
		b, e2 := strconv.Atoi(p[i+1:])
		if e1 != nil || e2 != nil {
			return 0, 0, false
		}
		return a, b, true
	}
	v, e := strconv.Atoi(p)
	if e != nil {
		return 0, 0, false
	}
	return v, v, true
}
