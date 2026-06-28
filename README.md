# Squint 🦝🐾

> ## **Squint is a GPU-aware Slurm monitor for your terminal. Read-only, zero-config, runs anywhere.**


![Squint Cover](./img/cover.png)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)


It's read-only by design: it only ever runs `squeue` / `scontrol`, plus `nvidia-smi` for GPU telemetry (DCGM next). Nothing to trust it with, nothing it can break. Point it at a cluster and look.

### ⚙️ Install
[![🚀 Release](https://github.com/hiteshsahu/squint/actions/workflows/release.yaml/badge.svg)](https://github.com/hiteshsahu/squint/actions/workflows/release.yaml)
[![🛠️ Build & Test](https://github.com/hiteshsahu/squint/actions/workflows/build-test.yaml/badge.svg)](https://github.com/hiteshsahu/squint/actions/workflows/build-test.yaml)

```bash
    # Install Squint
    brew install hiteshsahu/tap/squint   # (planned)
    
    go install github.com/hiteshsahu/squint@latest

```

### ▶️ Run

### With Mocks

No cluster handy? It ships with a mock source, so `squint` runs on your laptop out of the box.

```bash
    squint          # mock data — runs anywhere
```

![Squint Dashboard Mock](./img/result.jpeg)


Keys: 
- <kbd>q</kbd> quit 
- <kbd>r</kbd> refresh (auto refresh every 2s)
- <kbd>↑</kbd>/<kbd>↓</kbd>/<kbd>PgUp</kbd>/<kbd>PgDn</kbd>/wheel to scroll. 


### With Live Slurm Cluster


```bash
    squint --live   # read your real Slurm cluster
```

![](./img/live-dashboard.jpg)

### No real cluster to point `--live` at? 

[Caravan](https://github.com/hiteshsahu/caravan) bundles a real local Slurm cluster (controller + GPU compute nodes) into one CLI

**macOS / Linux**
```bash
     # start a local Slurm cluster in Docker/Podman
    caravan cluster up     
    # run a gpu workload
    caravan submit workloads\long_running_example.sh
    # monitor jobs                   
    squint --live
```

**Windows (PowerShell)**
```powershell
    .\caravan.exe cluster up
    $env:GOOS = "linux"; $env:GOARCH = "amd64"; go build -o squint.linux .
    docker cp squint.linux slurmctld:/usr/local/bin/squint
    docker exec -it slurmctld squint --live
```


---

## Why squint

> Because cluster state is often hidden behind pages of `squeue` output.

Squint turns that into something you can understand at a glance.


Slurm is powerful but feels like infrastructure from another era `squeue` is hard to read, "why is my job pending?" is a dark art, and nothing shows you what the GPUs are *actually doing*. `squint` is the live, GPU-native view of your cluster, in your shell, right now.

Two things nothing else in the Slurm ecosystem does well:

### 1. Find the Squatter GPU  
Every GPU on every node, colored by utilization and mapped to the job that owns it so the eight A100s allocated to a job using none of them light up red.

**GPU heatmap with idle-shaming.**

![Squint](./img/heatmap.jpeg)

That's the "who's squatting?" view every platform team wants and no Slurm tool gives them.

### 2. **"Why is my job pending?" in plain English.** 
`Reason=(QOSMaxGRESPerUser)` becomes *"You've hit your GPU quota for this QOS — wait for a running job to finish, or submit somewhere with more headroom."* For every common reason code, with the fix.

![Squint](./img/dashboard.jpeg)

---

## 👨‍💻 DEVELOP

- Requires **Go 1.22+**

###  ⚙️ Install dependencies
```bash
    # Install dependencies
    go mod tidy
```

###  🧪 Build & Test

Tests are run as part of CI itself.
    
 ```bash  
    # Build 
    go build ./...
    
    # Formatting go file
    gofmt -w . 
    
    # Linting
    go vet ./... 
    
```

### ▶️ Run 

###  Mock Run
- Run with mock gpu data
- Any Machine: **macOS, Windows, Linux**

``` bash
    go run .
```

### Live SLURM Run
- Run with Real GPU data 
- SLURM must be installed — squeue/scontrol/slurmd/munge present.
- Only **Linux** or **Winodws with WSL**

``` bash                    
    go run . --live          # Run with real Slurm
    
```



Keys: `q` quit · `r` refresh. Polls every 2s.

### 🧪 Testing

Run test
```bash
go test ./... 

# Test with coverage                      
go test -race -coverprofile=coverage.out ./...

# See code coverage
go tool cover -func=coverage.out

```

---

## How Squint is Built


```text

            ┌────────────────────────────────────────────┐
            │                 Squint TUI                 │
            │   GPU heatmap • jobs panel • dashboard     │
            └──────────────────┬─────────────────────────┘
                               │
                               ▼
            ┌────────────────────────────────────────────┐
            │              Source Layer                  │
            │     Mock Source  •  Live Source            │
            └──────────────────┬─────────────────────────┘
                               │
                               ▼
            ┌────────────────────────────────────────────┐
            │           Queue & GPU Insights              │
            │  Pending-Reason Translation                │
            │  Job → Node → GPU Allocation                │
            └──────────────────┬─────────────────────────┘
                               │
                               ▼
            ┌────────────────────────────────────────────┐
            │                  Slurm                     │
            │      squeue • scontrol • nvidia-smi        │
            └────────────────────────────────────────────┘
            
```

## 📁 Folder structure

The Source interface is the whole seam: Mock and Live both implement it, and the TUI never knows which one it's talking to.

```bash

squint/
    ├── main.go                         # CLI entrypoint (--live flag, Bubble Tea program)
    ├── internal/
    │   ├── model/
    │   │   └── types.go                # Job, GPU, Node, Snapshot
    │   │
    │   ├── source/
    │   │   ├── source.go               # Source interface + Mock (runs anywhere)
    │   │   ├── live.go                 # Live: squeue/scontrol + job→node→GPU attribution
    │   │   ├── slurm.go                # squeue/scontrol output parsing
    │   │   ├── pending.go              # Slurm reason-code → plain-English translator
    │   │   └── telemetry.go            # nvidia-smi GPU telemetry
    │   │
    │   └── tui/
    │       ├── app.go                  # Bubble Tea model: poll, fetch, keys
    │       └── view.go                 # Lip Gloss rendering: heatmap + jobs panel
    │
    ├── go.mod
    ├── README.md
    └── LICENSE
```

The `Source` interface is the whole seam: 
- `Mock` — deterministic demo data, runs anywhere, no Slurm needed.
- `Live` — real `squeue`/`scontrol`, plus `nvidia-smi` telemetry when available (DCGM next).
- The TUI never knows the difference.


---

## 🗺️ Roadmap

`squint` grows up one rung at a time — each earns the right to the next.

- **L0 — Observe** *(here)* — read-only GPU-aware TUI.
- **L1 — Act** — cancel / hold / requeue from the TUI; job-done desktop & Slack pings.
- **L2 — Declare** — clean job specs with pre-submit validation; `squint rerun <jobid>`.
- **L3 — API** — a friendly daemon over Slurm with a real job-state event stream.

It also emits Prometheus metrics, so it can feed a longer-term observability stack rather than replace one.

---

## Contributing

Issues, ideas, and pull requests are welcome.

If Squint helped you understand your cluster a little faster, consider giving the project a ⭐.

---

## License
*© 2026 [Hitesh Kumar Sahu](https://hiteshsahu.com) · Licensed under [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0)*

