# coco-iam-deploy

YAML-driven, engine-pluggable deployment tool for coco-iam.

A single binary reads a YAML file describing what to ship, where, and
how — then drives the chosen **engine** through a fixed lifecycle:

```
Preflight → UploadArtifacts → RenderEnv → StartProcesses → HealthCheck
                                     ↓ (on failure)
                                  Rollback
```

The engine layer is domain-agnostic: it knows nothing about
"frontend" or "backend" or any coco-iam specifics. It sees a
**Release** composed of **Artifacts** and **Processes**. Add a new
engine (AWS ECS, Kubernetes, fly.io, Cloud Run, …) without
changing the orchestrator or the CLI.

## Quick start

```sh
# From the deploy/app/ directory:
make build                       # builds bin/deploy
./bin/deploy validate \
    --config examples/coco-iam.yaml

# Real deploy (needs a reachable target):
./bin/deploy deploy --config examples/coco-iam.yaml
```

## Subcommands

| Command    | What it does                                                |
|------------|-------------------------------------------------------------|
| `deploy`   | Full Preflight → Upload → Env → Start → Health lifecycle    |
| `status`   | Prints current + previous release info from the target      |
| `rollback` | Swaps `current` back to the previous release, restarts      |
| `validate` | Parses + validates YAML + resolves secrets — no connection  |

Flags:

```
--config <path>    deploy YAML (default: deploy.yaml)
--no-rollback      (deploy only) skip Engine.Rollback on failure
```

## Configuration

See [`examples/coco-iam.yaml`](examples/coco-iam.yaml) for an
annotated reference. Top-level sections:

- `engine:` — discriminator (e.g. `ssh`)
- `<engine>:` — engine-specific typed config (e.g. `ssh:`)
- `release:` — `name`, `version`
- `artifacts:` — files / directories / archives / images to ship
- `processes:` — long-running things to (re)start
- `env:` — key/value with optional `literal:` / `env:` / `file:`
  prefixes for secret resolution
- `health_check:` — post-deploy HTTP probe
- `hooks:` — `pre_deploy` + `post_deploy` shell commands run
  **locally** before/after the deploy

## Engines

### `ssh` — single-host SSH + SFTP + systemd

Deploys atomically via the Capistrano pattern:

```
<target_dir>/
├── releases/
│   ├── 20260424T120000Z-abc123/
│   └── 20260424T123412Z-def456/
├── current -> releases/20260424T123412Z-def456
└── shared/
    └── .env
```

- Each release lives in its own timestamped directory
- `current` is a symlink swapped atomically (`ln -sfn`)
- Rollback is instant — just swaps `current` back
- `shared/.env` is rendered once per deploy

### Adding a new engine

1. Create `internal/engine/<name>/`
2. Implement `engine.Engine`
3. Self-register in `init()`:

   ```go
   engine.Register("ecs", func(raw *yaml.Node) (engine.Engine, error) {
       cfg, err := parseConfig(raw)
       if err != nil {
           return nil, err
       }
       return New(cfg), nil
   })
   ```

4. Import the package (with `_`) from `cmd/deploy/main.go` so
   its `init()` runs.

The orchestrator, runner, and CLI require zero changes.

## Layout

```
deploy/app/
├── cmd/deploy/               # CLI entry point
├── internal/
│   ├── config/               # YAML loader + validation
│   ├── engine/               # Engine interface + registry
│   │   ├── mock/             # test-only engine
│   │   └── ssh/              # SSH + SFTP engine
│   ├── runner/               # lifecycle orchestrator
│   └── secrets/              # literal: / env: / file: resolver
├── examples/                 # annotated example configs
├── Makefile
└── README.md
```

## Testing

```sh
make test        # unit tests for every package
make vet         # go vet ./...
```
