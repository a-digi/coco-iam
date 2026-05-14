# Makefile and CLI Reference

This document covers every `make` target and every command accepted by the compiled binary.

---

## Make targets

| Target | Description | Example |
|--------|-------------|---------|
| `build` | Compile the Go backend and write the binary to `./versions/coco-iam-<branch>`, where `<branch>` is the current Git branch with dots replaced by hyphens | `make build` |
| `run-dev` | Start the Go API server in the background using `go run` (no build step required) | `make run-dev` |
| `run-dev-app` | Start both the Go API server and the React development server. Runs `npm install` before starting Vite | `make run-dev-app` |
| `stop-dev` | Gracefully stop the background Go server started by `run-dev` or `run-dev-app` | `make stop-dev` |
| `run` | Start the compiled binary in the background. Requires a prior `make build` | `make run` |
| `stop` | Gracefully stop the server started by `make run` | `make stop` |
| `create-admin-dev` | Create a superadmin user using `go run` (edit the Makefile to supply real credentials before running) | `make create-admin-dev` |
| `create-admin` | Create a superadmin user using the compiled binary (edit the Makefile to supply real credentials before running) | `make create-admin` |
| `check-port` | Check whether a given port is in use. Requires the `PORT` variable | `make check-port PORT=2026` |
| `lint-fix` | Run ESLint with `--fix` on the React frontend source files | `make lint-fix` |
| `api-mod-tidy-vendor` | Run `go mod tidy` and `go mod vendor` inside the `api/` directory to keep Go dependencies consistent | `make api-mod-tidy-vendor` |
| `get-branch` | Print the current Git branch name with dots replaced by hyphens (the same suffix used in binary names) | `make get-branch` |
| `upgrade-go` | Upgrade the local Go installation via Homebrew | `make upgrade-go` |

### Notes on `create-admin-dev` and `create-admin`

Both targets contain literal placeholder strings `[USERNAME]`, `[EMAIL_ADDRESS]`, and `[PASSWORD]` in the Makefile. You must edit the Makefile or call the binary directly to provide real values. The targets are intended as a reference for the required argument order, not as ready-to-run commands with interactive prompts.

---

## Binary CLI commands

After running `make build`, the binary is available at `./versions/coco-iam-<branch>`. The binary accepts one positional argument (the action) and an optional second argument for a custom config file path.

```
./versions/coco-iam-<branch> <action> [config-path]
```

If `config-path` is omitted, the binary reads `config.json` from the current working directory.

### `start`

```sh
./versions/coco-iam-0-0-1 start
# or with a custom config path:
./versions/coco-iam-0-0-1 start /etc/coco-iam/config.json
```

Performs the following steps in order:

1. Initialises the file-based logger under `data/logs/`.
2. Extracts embedded SQL migration files to a temporary directory.
3. Opens (or creates) the SQLite database at `data/db/users.db`.
4. Runs pending migrations via `SyncMigrations`.
5. Verifies that at least one superadmin user exists; exits with an error if none is found.
6. Initialises the dependency injection container (`ContextBag`) with the database manager and logger.
7. Registers all routes from the embedded YAML configuration.
8. Starts the HTTP server on the port defined in `config.json`.
9. Writes the server PID to the file defined by `pid_file` in `config.json`.
10. Blocks until a shutdown signal is received, then performs a graceful shutdown.

### `shutdown`

```sh
./versions/coco-iam-0-0-1 shutdown
```

Reads the PID from the `pid_file` path in `config.json` and sends `SIGTERM` to that process. The running server completes any in-flight requests before exiting.

### `create-admin`

```sh
./versions/coco-iam-0-0-1 create-admin "<username>" "<email>" "<password>"
```

Creates a new superadmin user in the database. All three arguments are required and must not be empty.

The command:

1. Opens the database at `data/db/users.db` and runs any pending migrations.
2. Inserts a row into `admin_users` with `is_super_admin = true` and `is_active = true`.
3. Hashes the provided password with bcrypt and inserts the hash into `user_auth_password`.

The command exits with status 0 on success and prints an error message to stdout on failure. It does not start the HTTP server.

**Example:**

```sh
./versions/coco-iam-0-0-1 create-admin "alice" "alice@example.com" "hunter2"
```
