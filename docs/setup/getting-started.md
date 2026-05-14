# Getting Started

This guide walks you through cloning, configuring, and running coco-iam for local development.

---

## Prerequisites

| Tool | Minimum version | Notes |
|------|----------------|-------|
| Go | 1.26 | Required to build and run the backend |
| Make | Any recent version | Used for all development workflows |
| Git | Any recent version | Required to clone the repository |

Verify your versions before proceeding:

```sh
go version
make --version
git --version
```

---

## Clone and first build

```sh
git clone <repository-url> coco-iam
cd coco-iam
make build
```

`make build` compiles the Go backend and writes the binary to `./versions/` using the current branch name as a suffix. For example, on branch `0.0.1` the output binary is `./versions/coco-iam-0-0-1`.

The binary is self-contained: it embeds all SQL migration files and route configuration at compile time.

---

## Configuration

coco-iam reads a `config.json` file from the project root at startup. A minimal configuration looks like this:

```json
{
  "port": 2026,
  "pid_file": "./server.pid"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `port` | integer | Yes | The TCP port the HTTP server listens on |
| `pid_file` | string | Yes | Path where the server writes its PID on startup. Used by the shutdown command to send a termination signal |

The OAuth signing configuration (JWT secret, issuer, audience) is also read from `config.json` by the authentication layer. See `api/vendor/github.com/a-digi/coco-oauth/oauth/config.go` for the full set of fields recognised by the OAuth library.

The `config.json` at the project root is embedded into the compiled binary via `go:embed` (see `api/config/embed.go`). If you modify `config.json` after building, you must rebuild for the changes to take effect.

---

## Start development servers

To start both the Go API server and the React development server in a single command:

```sh
make run-dev-app
```

This target:
1. Starts the Go server in the background (`go run main.go start`) on port **2026**
2. Runs `npm install` in the `app/` directory
3. Starts the Vite development server for the React frontend

To start only the Go API server without the frontend:

```sh
make run-dev
```

The Go server process runs in the background. Its PID is written to the file specified by `pid_file` in `config.json` (default: `./server.pid`).

---

## Create the first admin user

Before you can log in, at least one superadmin user must exist in the database. The server refuses to start if no superadmin is found.

The `make create-admin-dev` target reads credentials from a `.env` file at the project root. Fill in the three variables before running:

```sh
# .env
ADMIN_USERNAME=alice
ADMIN_EMAIL=alice@example.com
ADMIN_PASSWORD=s3cr3t
```

Then run:

```sh
make create-admin-dev
```

The target will fail with a clear error if `.env` is missing or any of the three variables are empty.

Alternatively, call the binary directly without the `.env` file:

```sh
cd api && go run main.go create-admin "<username>" "<email>" "<password>"
```

Or, if you have already built the binary:

```sh
./versions/coco-iam-0-0-1 create-admin "alice" "alice@example.com" "s3cr3t"
```

The command connects to the SQLite database, hashes the password with bcrypt, inserts a row into `admin_users`, and inserts the hashed password into `user_auth_password`. The user is created with `is_super_admin = true` and `is_active = true`.

---

## Accessing the application

| Service | URL |
|---------|-----|
| React frontend (dev server) | http://localhost:5173 |
| Go API | http://localhost:2026 |
| API health check | http://localhost:2026/ |

A `GET /` request returns the plain-text response `Server is running!` when the backend is up.

The API base path for all authenticated endpoints is `/api/v1/admin/`. Authentication uses Bearer tokens obtained from `POST /api/v1/admin/oauth/authenticate`.

---

## Runtime data directories

coco-iam writes all persistent data under the `data/` directory at the project root. This directory is created automatically on first startup.

```
data/
├── db/
│   └── users.db          # SQLite database file
└── logs/
    └── YYYY/
        └── MM/
            └── DD/
                └── server_*.log   # Daily log files
```

**`data/db/users.db`** — the SQLite database. Do not delete or move this file while the server is running.

**`data/logs/`** — log files are organised by date. Each server session appends to a file named `server_<timestamp>.log` under the appropriate year/month/day subdirectory. Log files are not rotated automatically; archive or truncate them manually as needed.

Neither directory should be committed to version control. Add them to `.gitignore` if they are not already excluded.

---

## Stopping the server

```sh
make stop-dev
```

This sends a `SIGTERM` to the process whose PID is recorded in the `pid_file` specified in `config.json`. The server performs a graceful shutdown, waiting for in-flight requests to complete before exiting.

If you built the binary and started it with `make run`, use:

```sh
make stop
```

Both targets work the same way internally — they run the binary with the `shutdown` argument, which reads the PID file and sends the signal.
