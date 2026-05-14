# Observability (coco-observe)

coco-observe is the built-in system observability layer for coco-iam. It collects Go runtime metrics and OS-level statistics from remote servers, stores them in a local SQLite database, and displays them in the admin UI.

---

## Table of contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Build](#build)
- [Configuration](#configuration)
- [Agent management](#agent-management)
- [Running the agent](#running-the-agent)
- [Metrics reference](#metrics-reference)
- [API reference](#api-reference)
- [Scopes](#scopes)

---

## Overview

```
Remote server                     coco-iam server
┌─────────────────┐               ┌──────────────────────────────┐
│  observe-agent  │  HMAC push    │  /api/v1/admin/observe/push  │
│  (per-agent     │ ────────────► │  aggregator → SQLite         │
│   binary)       │               │                              │
└─────────────────┘               │  /api/v1/admin/observe/      │
                                  │  metrics   → admin UI        │
                                  └──────────────────────────────┘
```

Each remote server runs one `observe-agent` binary. The agent pushes metric batches over HTTPS to coco-iam on a configurable interval (default 180 s). coco-iam stores the data in a SQLite database and exposes it through the admin UI under **Observe**.

---

## Architecture

### Components

| Component | Location | Role |
|-----------|----------|------|
| `coco-observe` plugin | `plugins/coco-observe/` | Go module: agent + aggregator library |
| Aggregator | `plugins/coco-observe/aggregator/` | HTTP handlers, SQLite store, binary embedding |
| Agent | `plugins/coco-observe/cmd/agent/` | Metrics collector, push client, buffer |
| Admin UI | `app/src/Components/Observe/` | Agents table, metrics charts |

### Data flow

1. **Agent creation** — an admin registers a new agent in the UI. coco-iam generates credentials (API key + secret), produces a per-agent binary (base binary + embedded YAML config) for each architecture, and stores both binaries on disk. The paths are saved in the SQLite database.
2. **Binary download** — the admin downloads the binary for the target architecture directly from the UI. The file is self-contained: no config file is needed on the remote server.
3. **Metric push** — the agent runs on the remote server, collects metrics, and POSTs batches to `/api/v1/admin/observe/push`. Each request is authenticated with an HMAC-SHA256 signature derived from the agent's API secret.
4. **Storage** — coco-iam stores each batch in `observe-hot.db`. When the table exceeds 10 million rows it is vacuumed into a timestamped archive file and the hot table is cleared.
5. **Query** — the admin UI fetches metrics for a selected agent and time range. The aggregator queries the hot database and any relevant archive files.

### Embedded binary mechanism

The base agent binaries (linux/amd64 and linux/arm64) are cross-compiled on the developer machine and embedded into the coco-iam server binary using Go's `//go:embed` directive. At runtime, when an agent is created, coco-iam appends the agent-specific YAML config (API key, secret, push URL) after a magic delimiter in the binary:

```
[ ELF binary ] + [ \n---OBSERVE-CONFIG-V1---\n ] + [ YAML config ]
```

ELF/Mach-O loaders ignore trailing data, so the binary remains executable. The agent reads its own executable on startup, finds the delimiter, and parses the config block.

---

## Build

The agent binaries must be cross-compiled before building coco-iam so that they can be embedded.

```bash
# From the repo root — run once before each coco-iam release build
cd plugins/coco-observe && make build-agent-linux
```

This produces:

```
plugins/coco-observe/aggregator/binaries/observe-agent-linux-amd64
plugins/coco-observe/aggregator/binaries/observe-agent-linux-arm64
```

Then build coco-iam normally:

```bash
# Re-vendor (copies embedded binaries into api/vendor/)
cd api && go mod vendor

# Build server binary (embeds agent binaries via //go:embed)
make build-linux
```

The deployed coco-iam binary is self-contained. No agent binaries need to be shipped separately.

---

## Configuration

### Server environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OBSERVE_DATA_DIR` | `./data/observe` | Where SQLite databases and per-agent binaries are stored |
| `COCO_IAM_PUBLIC_BASE_URL` | `http://localhost:2026` | Public base URL embedded as the push endpoint in agent configs |

### Agent config (embedded YAML)

The following fields are embedded into every agent binary at creation time:

```yaml
api_key: <generated>
api_secret: <generated>
aggregator_url: https://your-coco-iam.example.com/api/v1/admin/observe/push
push_interval: 180s
buffer_dir: /var/lib/observe-agent/buffer
buffer_retention: 24h
processes: []
```

`processes` is an optional list of Go services to scrape runtime metrics from (via `/debug/vars`):

```yaml
processes:
  - name: my-service
    scrape_url: http://localhost:8080/debug/vars
```

All fields can also be overridden at runtime via environment variables:

| Variable | Overrides |
|----------|-----------|
| `OBSERVE_API_KEY` | `api_key` |
| `OBSERVE_API_SECRET` | `api_secret` |
| `OBSERVE_AGGREGATOR_URL` | `aggregator_url` |
| `OBSERVE_BUFFER_DIR` | `buffer_dir` |

---

## Agent management

### Register an agent

1. Open the admin UI and navigate to **Observe → Agents**.
2. Click **+ Register**.
3. Enter a name (e.g. `production-server-01`).
4. Copy the API key and secret — the secret is shown exactly once.
5. The server immediately generates per-agent binaries for both architectures.

### Download the binary

1. In the Agents table, click **Download ▾** on the agent row.
2. Select the architecture matching your remote server:
   - **Linux amd64** — Intel/AMD (most VPS and cloud VMs: x86_64)
   - **Linux arm64** — ARM (AWS Graviton, Oracle Ampere, Raspberry Pi 4+)
3. The downloaded binary already contains the agent's credentials and push URL. No additional configuration is needed.

To confirm the architecture of a remote server:

```bash
uname -m
# x86_64  → amd64
# aarch64 → arm64
```

### Delete an agent

Click **Delete** on the agent row. This removes the DB record, all stored metric batches, and the per-agent binary files from disk.

---

## Running the agent

### Quick start (manual)

```bash
# Copy to remote server
scp observe-agent-amd64-production-server-01 user@host:/opt/observe-agent/agent

# Run
ssh user@host "chmod +x /opt/observe-agent/agent && /opt/observe-agent/agent"
```

### As a systemd service

Create `/etc/systemd/system/observe-agent.service`:

```ini
[Unit]
Description=coco-observe agent
After=network.target

[Service]
ExecStart=/opt/observe-agent/agent
Restart=on-failure
RestartSec=15s

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable --now observe-agent
journalctl -u observe-agent -f
```

### Verifying connectivity

The agent logs push results on startup and after each interval:

```
observe agent: using embedded config
observe agent: starting (push interval: 3m0s)
observe agent: pushed batch seq=1 metrics=12
```

If the push fails, the batch is written to the buffer directory and retried on the next interval (up to `buffer_retention`, default 24 h).

---

## Metrics reference

Each push batch contains a JSON payload with the following structure:

### OS metrics

| Field | Description |
|-------|-------------|
| `hostname` | Server hostname |
| `os` | Operating system |
| `arch` | CPU architecture |
| `cpu_count` | Number of logical CPUs |
| `uptime_seconds` | System uptime |
| `mem_total_bytes` | Total RAM |
| `mem_used_bytes` | Used RAM |
| `mem_available_bytes` | Available RAM |
| `load_1`, `load_5`, `load_15` | Load averages |
| `disk` | Array of mount point stats (path, total, used, free bytes) |
| `net` | Array of interface stats (name, bytes sent/received) |

### Go runtime metrics (per process)

Collected when `processes` is configured and the service exposes `/debug/vars`.

| Field | Description |
|-------|-------------|
| `goroutines` | Number of goroutines |
| `heap_alloc_bytes` | Bytes currently allocated on the heap |
| `heap_inuse_bytes` | Bytes in in-use heap spans |
| `heap_sys_bytes` | Bytes obtained from the OS |
| `gc_pause_ns` | Last GC stop-the-world pause (nanoseconds) |
| `gc_count` | Total number of GCs |
| `next_gc_bytes` | Target heap size for next GC |

---

## API reference

All endpoints are under `/api/v1/admin/`.

### POST /observe/push

Accepts a metric batch from an agent. Authenticated via HMAC-SHA256 — no session token required.

**Headers**

| Header | Value |
|--------|-------|
| `X-Agent-Key` | Agent API key |
| `X-Agent-Sig` | HMAC-SHA256 signature of the request body |
| `Content-Type` | `application/json` |

**Response** `200 OK`

### GET /observe/metrics

Query stored metric batches for an agent.

**Query parameters**

| Parameter | Required | Description |
|-----------|----------|-------------|
| `agent_id` | yes | Agent ID |
| `range` | no | `1h`, `6h`, `24h`, `7d` (default `1h`) |
| `from` | no | RFC3339 start time (overrides `range`) |
| `to` | no | RFC3339 end time |
| `limit` | no | Max rows returned (default 500) |

**Response** `200 OK` — array of batch objects.

### GET /observe/metrics/latest

Returns the single most-recent batch for `agent_id`.

### GET /observe/agents

Returns all registered agents. Never includes the API secret.

### POST /observe/agents

Registers a new agent. Generates credentials and per-agent binaries.

**Body**
```json
{ "name": "production-server-01" }
```

**Response** `201 Created`
```json
{
  "id": "...",
  "name": "production-server-01",
  "api_key": "...",
  "api_secret": "..."
}
```

The `api_secret` is returned exactly once and never stored in plaintext.

### DELETE /observe/agents/{id}

Deletes the agent, all its metric batches, and the stored per-agent binary files.

### GET /observe/agents/{id}/download?arch=amd64|arm64

Downloads the pre-generated per-agent binary for the specified architecture.

---

## Scopes

| Scope | Grants |
|-------|--------|
| `observe:view` | Read metrics (`GET /observe/metrics`) |
| `observe:manage` | Manage agents and download binaries |
| `super:admin` | All of the above |
