# Smoke tests

End-to-end HTTP tests that drive a **running coco-iam server** through
real admin + machine-auth flows. Opt-in via the `smoke` build tag so
they don't slow down `go test ./...`.

## Running locally

1. Start the dev server in one terminal:

   ```sh
   make run-dev
   ```

2. Ensure a superadmin account exists (once-only):

   ```sh
   make create-admin-dev
   # or inline:
   #   cd api && go run main.go create-admin "<user>" "<email>" "<pass>"
   ```

3. Run the smoke tests with admin credentials in env:

   ```sh
   COCO_IAM_ADMIN_USER="<user>" \
   COCO_IAM_ADMIN_PASS="<pass>" \
   make test-smoke
   ```

   Or directly:

   ```sh
   COCO_IAM_ADMIN_USER=... COCO_IAM_ADMIN_PASS=... \
     go test -tags smoke ./api/tests/smoke/... -v
   ```

4. To point at a non-default host/port, set `COCO_IAM_URL`:

   ```sh
   COCO_IAM_URL="https://staging.example.com" \
   COCO_IAM_ADMIN_USER=... COCO_IAM_ADMIN_PASS=... make test-smoke
   ```

## Skipping behaviour

Each test calls `requireAdmin(t)`, which invokes `t.Skip` when the
admin env vars are unset. That means `make test-smoke` on a clean
checkout — with no env configured — is a **pass** (every test
skipped). This is intentional: smoke tests are checks you run against
a known environment, not mandatory CI gates.

## What's covered

- `apicredentials_smoke_test.go` — admin creates org/workspace/app,
  issues an API credential, uses it against
  `/a/<slugs>/security-key` (public + private endpoints), tests
  negative cases (missing header, wrong secret, unknown api_id,
  cross-tenant), then revokes and confirms subsequent calls 401.

## Known footguns

- Fixtures are created with slugs like `smoke-org-<hhmmss.sss>`.
  Parallel smoke runs against the same server will race on slug
  uniqueness — for CI use a dedicated env.
- Cleanup is best-effort. If a test aborts before its cleanup hook
  runs, leftover orgs may accumulate — delete via the admin UI or
  drop the dev DB.
