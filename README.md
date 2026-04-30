# NVIMON

`nvimon` is a Go terminal application for monitoring NVIDIA GPUs across one or more Linux hosts. It is built as a small fleet monitoring stack:

- `nvimon`: Bubble Tea TUI for local and remote monitoring
- `nvimon-agent`: lightweight HTTP agent for remote hosts
- shared telemetry model, collectors, and transport code

The goal is to give you a compact, operator-focused view of GPU usage, GPU-only processes, CPU/RAM load, and short-window history charts without requiring a browser or a full observability stack.

## What It Shows

`nvimon` displays:

- per-host CPU usage
- per-host RAM usage
- per-host aggregate GPU usage bars
- per-GPU compute utilization
- per-GPU VRAM usage
- per-GPU temperature
- per-GPU fan speed
- per-GPU power draw
- per-GPU performance/profile state
- active GPU-using processes only
- short history graphs for the key GPU metrics
- host connection status and collector warnings

The UI is designed to stay usable on both wide and short terminals, and it supports mouse/touch selection for host switching.

## Architecture

```mermaid
flowchart LR
  subgraph Hosts
    L[Local Linux host]
    R1[Remote Linux host]
    R2[Remote Linux host]
  end

  subgraph nvimon-agent
    A1[HTTP API]
    A2[Local collector]
    A3[NVML or nvidia-smi]
    A4[/proc and gopsutil]
    A2 --> A3
    A2 --> A4
    A1 --> A2
  end

  subgraph nvimon
    TUI[Bubble Tea TUI]
    C1[Local collector]
    C2[HTTP client]
    H[History store]
    M[Normalized model]
    TUI --> M
    M --> H
    M --> C1
    M --> C2
  end

  L --> C1
  R1 --> A1
  R2 --> A1
  C2 --> A1
```

The core design choice is to normalize all telemetry into one snapshot model. The TUI does not care whether data came from the local machine or from a remote agent.

## Backends

The project supports two build/runtime paths for GPU telemetry:

- `local-nvml`: CGO-enabled build with NVIDIA NVML bindings
- `portable`: CGO-disabled build that falls back to `nvidia-smi`

The `local-nvml` build is best produced on the target host or on a builder with a compatible libc and CPU baseline.

The default `make build` produces both:

- `dist/portable/nvimon`
- `dist/portable/nvimon-agent`
- `dist/local-nvml/nvimon`
- `dist/local-nvml/nvimon-agent`

The installer chooses the artifact that can actually run on the target host.

## Data Collected

Per host:

- hostname
- connection state
- collector latency
- collector warnings
- CPU usage
- RAM used and total
- uptime
- load average when available
- total GPU count
- GPU backend name

Per GPU:

- index
- UUID
- product name
- VRAM used and total
- compute utilization
- temperature
- fan speed
- power draw
- power limit
- clock data when available
- pstate / profile

GPU processes:

- PID
- user
- command
- GPU index
- VRAM used by process
- per-process SM/utilization fields when available

Unavailable fields are shown as unknown rather than being faked as zero.

## Build

```bash
make build
```

This builds both variants into `dist/` and copies a default config to `dist/nvimon.config.yaml`.

Portable-only build:

```bash
make build-portable
```

CGO/NVML-enabled build:

```bash
make build-native
```

## Run

Run the TUI locally:

```bash
./dist/portable/nvimon
```

Run a single snapshot in text mode:

```bash
./dist/portable/nvimon --once
```

Emit JSON instead of the text summary:

```bash
./dist/portable/nvimon --once --json
```

Inspect a remote agent directly:

```bash
./dist/portable/nvimon --remote-snapshot http://host:9910 --remote-token TOKEN --json
```

Run the agent:

```bash
./dist/portable/nvimon-agent --config ./config.example.yaml
```

## Configuration

The main config file is YAML. The default path is `~/.config/nvimon/config.yaml`, or you can pass `--config`.

Example:

```yaml
refresh_interval: 1s
history_length: 120

agent:
  bind_address: 0.0.0.0:9910
  auth_token: ""

hosts:
  - name: local
    mode: local
  - name: gpu-a
    mode: remote
    url: http://10.0.0.25:9910
    token: ""
```

## Agent Install

There is a simple Linux installer for the agent:

```bash
./scripts/install-agent.sh
```

It:

- looks for the built agent binaries under `dist/`
- prefers `dist/local-nvml/nvimon-agent` if it can run on the host
- falls back to `dist/portable/nvimon-agent`
- installs the binary to `/usr/local/bin`
- installs a systemd service
- enables and restarts the service
- updates the install safely when a newer binary is present

The systemd unit template lives in `packaging/systemd/nvimon-agent.service`.

## UI Controls

- `0` selects all hosts
- `1-9` selects a specific host
- `j` / `k` cycles host scope
- `x` toggles the GPU process pane
- `/` starts process filtering
- `w` opens the warnings dialog
- `g` cycles aggregate mode
- `p` pauses refresh
- `r` refreshes immediately
- `?` toggles help
- click or tap a host row to select it

## Repository Layout

- `cmd/nvimon`: TUI entrypoint
- `cmd/nvimon-agent`: agent entrypoint
- `internal/collector`: local telemetry collection
- `internal/model`: normalized snapshot structs and formatting
- `internal/history`: ring buffers and time-series storage
- `internal/transport/httpapi`: agent HTTP server and client
- `internal/tui`: Bubble Tea UI and rendering
- `scripts/install-agent.sh`: Linux agent install/update script
- `packaging/systemd/nvimon-agent.service`: systemd unit template

## Status

The project is functional now and evolving toward a full multi-host GPU monitoring tool. The current focus is improving density, usability, and deployment ergonomics without losing the compact terminal-first workflow.
