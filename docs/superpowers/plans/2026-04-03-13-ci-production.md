# CI Pipeline & Production Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Set up GitHub Actions CI workflows (backend + frontend), create the `.env.example` file, configure Docker log rotation, and verify the complete production checklist from `design/06-operations.md`.

**Architecture:** Two independent GitHub Actions workflows (path-filtered to avoid wasteful runs). No automatic deployment — this is a personal self-hosted project. Docker Compose is the deployment unit.

**Tech Stack:** GitHub Actions, Go 1.23, Node.js 22, Docker, postgres:17-alpine, golang-migrate, govulncheck, `npm audit`

---

## Files

| File                             | Role                                                             |
| -------------------------------- | ---------------------------------------------------------------- |
| `.github/workflows/backend.yml`  | Backend CI: build, vet, migrate, test, govulncheck, Docker build |
| `.github/workflows/frontend.yml` | Frontend CI: install, type-check, build, npm audit, Docker build |
| `.env.example`                   | All required environment variables with safe placeholder values  |
| `docker-compose.yml`             | Production Docker Compose (verify log driver config)             |
| `docker-compose.override.yml`    | Dev overrides (Mailpit, volume mounts)                           |

---

## Task 1: Backend CI Workflow

**Files:**

- Create: `.github/workflows/backend.yml`

- [ ] **Step 1: Create the GitHub Actions directory**

```bash
mkdir -p .github/workflows
```

- [ ] **Step 2: Write the backend workflow**

```yaml
# .github/workflows/backend.yml
name: Backend CI

on:
  push:
    paths:
      - 'backend/**'
      - '.github/workflows/backend.yml'
  pull_request:
    paths:
      - 'backend/**'

jobs:
  build-and-test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:17-alpine
        env:
          POSTGRES_DB: fabyoumeme_test
          POSTGRES_USER: fabyoumeme
          POSTGRES_PASSWORD: testpassword
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 5s
          --health-retries 5

    env:
      DATABASE_URL: postgres://fabyoumeme:testpassword@localhost:5432/fabyoumeme_test

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'
          cache: true
          cache-dependency-path: backend/go.sum

      - name: Download modules
        working-directory: backend
        run: go mod download

      - name: Build
        working-directory: backend
        run: go build ./...

      - name: Vet
        working-directory: backend
        run: go vet ./...

      - name: Install golang-migrate
        run: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

      - name: Run migrations
        working-directory: backend
        run: migrate -path ./db/migrations -database "$DATABASE_URL" up

      - name: Test
        working-directory: backend
        run: go test -race -count=1 ./...

      - name: Vulnerability check
        working-directory: backend
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...

      - name: Docker build smoke test
        run: docker build -t fabyoumeme-backend:ci ./backend
```

- [ ] **Step 3: Verify the workflow file is valid YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/backend.yml'))" && echo "YAML valid"
```

Expected: `YAML valid`

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/backend.yml
git commit -m "ci: add backend workflow (build/vet/migrate/test/govulncheck/docker)"
```

---

## Task 2: Frontend CI Workflow

**Files:**

- Create: `.github/workflows/frontend.yml`

- [ ] **Step 1: Write the frontend workflow**

```yaml
# .github/workflows/frontend.yml
name: Frontend CI

on:
  push:
    paths:
      - 'frontend/**'
      - '.github/workflows/frontend.yml'
  pull_request:
    paths:
      - 'frontend/**'

jobs:
  build-and-check:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '22'
          cache: 'npm'
          cache-dependency-path: frontend/package-lock.json

      - name: Install dependencies
        working-directory: frontend
        run: npm ci

      - name: Type check
        working-directory: frontend
        run: npm run check

      - name: Build
        working-directory: frontend
        env:
          PUBLIC_API_URL: http://localhost:8080
        run: npm run build

      - name: Audit dependencies
        working-directory: frontend
        # --audit-level=high: fail only on high or critical vulns
        run: npm audit --audit-level=high

      - name: Docker build smoke test
        run: docker build -t fabyoumeme-frontend:ci ./frontend
```

- [ ] **Step 2: Verify the workflow file is valid YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/frontend.yml'))" && echo "YAML valid"
```

Expected: `YAML valid`

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/frontend.yml
git commit -m "ci: add frontend workflow (install/type-check/build/audit/docker)"
```

---

## Task 3: Environment Variables Template

**Files:**

- Create: `.env.example`

- [ ] **Step 1: Write the env example file**

```bash
# .env.example
# Copy this file to .env and fill in all required values before running.
# Never commit the .env file — it contains secrets.

# ─── Database ──────────────────────────────────────────────────────────────────
POSTGRES_PASSWORD=change_me_strong_password

# ─── Backend ───────────────────────────────────────────────────────────────────
# Public URL of the backend API (used by frontend SSR to reach the backend)
BACKEND_URL=http://backend:8080

# ─── Frontend ──────────────────────────────────────────────────────────────────
# Public URL that users see in their browser (used for magic links and CORS)
FRONTEND_URL=https://yourdomain.example.com

# ─── RustFS / S3-compatible object storage ─────────────────────────────────────
# External RustFS stack endpoint (e.g., http://rustfs:9000)
RUSTFS_ENDPOINT=http://rustfs:9000
RUSTFS_ACCESS_KEY=your_rustfs_access_key
RUSTFS_SECRET_KEY=your_rustfs_secret_key
RUSTFS_BUCKET=fabyoumeme-assets

# ─── SMTP ──────────────────────────────────────────────────────────────────────
# Production SMTP credentials. Dev override sets these to Mailpit via override.yml
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USERNAME=your_smtp_username
SMTP_PASSWORD=your_smtp_password
# From address shown to recipients
SMTP_FROM=noreply@yourdomain.example.com

# ─── Admin seed ────────────────────────────────────────────────────────────────
# Email address for the first admin account (created automatically on startup).
# Idempotent — safe to leave set on subsequent restarts.
SEED_ADMIN_EMAIL=admin@yourdomain.example.com

# ─── Tuning (optional — defaults shown) ────────────────────────────────────────
BACKEND_PORT=8080
LOG_LEVEL=info

# Session and magic link TTL
SESSION_TTL=720h
MAGIC_LINK_TTL=15m

# WebSocket settings
RECONNECT_GRACE_WINDOW=30s
WS_RATE_LIMIT=20
WS_READ_LIMIT_BYTES=4096
WS_READ_DEADLINE=60s
WS_PING_INTERVAL=25s

# Upload size limit in bytes (default: 2 MB)
MAX_UPLOAD_SIZE_BYTES=2097152

# Rate limits
RATE_LIMIT_AUTH_RPM=10
RATE_LIMIT_INVITE_VALIDATION_RPH=20
RATE_LIMIT_ROOMS_RPH=10
RATE_LIMIT_UPLOADS_RPH=50
RATE_LIMIT_GLOBAL_RPM=100
```

- [ ] **Step 2: Verify `.env` is in `.gitignore`**

```bash
grep -q "^\.env$" .gitignore && echo "OK" || echo "ADD .env TO .gitignore NOW"
```

If not present, add it:

```bash
echo ".env" >> .gitignore
```

- [ ] **Step 3: Commit**

```bash
git add .env.example .gitignore
git commit -m "chore: add .env.example with all required variables documented"
```

---

## Task 4: Docker Compose Log Rotation

**Files:**

- Modify: `docker-compose.yml`

- [ ] **Step 1: Read the current docker-compose.yml**

```bash
cat docker-compose.yml
```

- [ ] **Step 2: Add log driver config to each service**

Add to each service (`postgres`, `backend`, `frontend`) a `logging` block:

```yaml
logging:
  driver: json-file
  options:
    max-size: '50m'
    max-file: '5'
```

This caps logs at 50 MB per file × 5 rotations = 250 MB max per service.

- [ ] **Step 3: Verify the updated docker-compose.yml is valid**

```bash
docker compose config > /dev/null && echo "Compose config valid"
```

Expected: `Compose config valid`

- [ ] **Step 4: Commit**

```bash
git add docker-compose.yml
git commit -m "chore: add Docker log rotation (50MB×5 per service) per GDPR log retention"
```

---

## Task 5: Production Checklist Verification Script

**Files:**

- Create: `scripts/check-production-readiness.sh`

- [ ] **Step 1: Create the scripts directory**

```bash
mkdir -p scripts
```

- [ ] **Step 2: Write the readiness check script**

```bash
#!/usr/bin/env bash
# scripts/check-production-readiness.sh
# Run this before inviting the first users.
# Exits 0 if all checks pass; exits 1 on any failure.

set -euo pipefail

PASS=0
FAIL=0

check() {
  local label="$1"
  local cmd="$2"
  if eval "$cmd" &>/dev/null; then
    echo "  ✓ $label"
    ((PASS++)) || true
  else
    echo "  ✗ $label  ← FAILED"
    ((FAIL++)) || true
  fi
}

echo ""
echo "FabDoYouMeme — Production Readiness Check"
echo "=========================================="
echo ""

echo "Environment:"
check ".env file exists" "test -f .env"
check "POSTGRES_PASSWORD set" "grep -q 'POSTGRES_PASSWORD=' .env && ! grep -q 'POSTGRES_PASSWORD=change_me' .env"
check "FRONTEND_URL set (not localhost)" "grep -q 'FRONTEND_URL=https://' .env"
check "SEED_ADMIN_EMAIL set" "grep -qE 'SEED_ADMIN_EMAIL=.+@.+' .env"
check "SMTP_HOST set" "grep -qE 'SMTP_HOST=.{3,}' .env && ! grep -q 'SMTP_HOST=mailpit' .env"
check "RUSTFS_ENDPOINT set" "grep -qE 'RUSTFS_ENDPOINT=http' .env"

echo ""
echo "Security:"
check ".env not committed" "! git ls-files --error-unmatch .env 2>/dev/null"
check "Secrets not in CLAUDE.md" "! grep -qi 'password\|secret_key\|access_key' CLAUDE.md"

echo ""
echo "Backend:"
check "Go tests pass" "cd backend && go test -race -count=1 ./... 2>&1 | tail -1 | grep -qE 'ok|no test files'"
check "govulncheck passes" "cd backend && govulncheck ./... 2>&1 | tail -1 | grep -q 'No vulnerabilities found'"

echo ""
echo "Frontend:"
check "npm audit passes" "cd frontend && npm audit --audit-level=high"

echo ""
echo "Docker:"
check "docker-compose.yml valid" "docker compose config > /dev/null"
check "Log rotation configured" "grep -q 'max-size' docker-compose.yml"

echo ""
echo "──────────────────────────────────────────"
echo "Results: $PASS passed, $FAIL failed"
echo ""

if [ "$FAIL" -gt 0 ]; then
  echo "Fix the failing items above before going live."
  exit 1
else
  echo "All checks passed. Review the manual items in design/06-operations.md before inviting users."
  exit 0
fi
```

- [ ] **Step 3: Make the script executable**

```bash
chmod +x scripts/check-production-readiness.sh
```

- [ ] **Step 4: Run the script in dry-run mode**

```bash
bash scripts/check-production-readiness.sh
```

Expected: Some items will fail (e.g., `.env` doesn't exist yet, backend tests not ready). That's fine — the script is for pre-launch verification. Confirm the script runs without crashing.

- [ ] **Step 5: Commit**

```bash
git add scripts/check-production-readiness.sh
git commit -m "chore: add production readiness check script"
```

---

## Task 6: Backup Cron Script

**Files:**

- Create: `scripts/backup.sh`

- [ ] **Step 1: Write the backup script**

```bash
#!/usr/bin/env bash
# scripts/backup.sh
# Run nightly via host cron: 0 2 * * * /path/to/FabDoYouMeme/scripts/backup.sh
# Requires: BACKUP_DIR env var (or defaults to /var/backups/fabyoumeme)
#           COMPOSE_FILE env var pointing to docker-compose.yml location

set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-$(dirname "$0")/../docker-compose.yml}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/fabyoumeme}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/postgres_$TIMESTAMP.sql.gz"

mkdir -p "$BACKUP_DIR"

echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] Starting backup → $BACKUP_FILE"

docker compose -f "$COMPOSE_FILE" exec -T postgres \
  pg_dump -U fabyoumeme fabyoumeme \
  | gzip > "$BACKUP_FILE"

echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] Backup complete: $(du -sh "$BACKUP_FILE" | cut -f1)"

# Retain only the last 7 backups (GDPR: 7-day retention)
find "$BACKUP_DIR" -name 'postgres_*.sql.gz' -mtime +7 -delete
echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] Old backups pruned (>7 days)"
```

- [ ] **Step 2: Make the script executable**

```bash
chmod +x scripts/backup.sh
```

- [ ] **Step 3: Verify the script is syntactically correct**

```bash
bash -n scripts/backup.sh && echo "Syntax OK"
```

Expected: `Syntax OK`

- [ ] **Step 4: Commit**

```bash
git add scripts/backup.sh
git commit -m "chore: add nightly PostgreSQL backup script with 7-day retention"
```

---

## Task 7: Metrics Security Verification

The `/api/metrics` endpoint (Prometheus) must never be publicly accessible.

- [ ] **Step 1: Verify metrics is IP-restricted in the backend middleware**

Search for the metrics route registration to confirm the IP-allow middleware is applied:

```bash
grep -r "metrics" backend/cmd/server/main.go
```

Expected output contains something like:

```
r.With(middleware.RequireLocalOrTrustedIP).Get("/api/metrics", ...)
```

If the middleware is absent, add it. The middleware must only allow `127.0.0.1` and the private Docker subnet (`172.16.0.0/12`) to access `/api/metrics`.

The middleware (add to `backend/internal/middleware/ip_allowlist.go` if not already present):

```go
package middleware

import (
  "net"
  "net/http"
)

var allowedNets = []*net.IPNet{
  mustParseCIDR("127.0.0.0/8"),
  mustParseCIDR("10.0.0.0/8"),
  mustParseCIDR("172.16.0.0/12"),
  mustParseCIDR("192.168.0.0/16"),
}

func mustParseCIDR(s string) *net.IPNet {
  _, n, err := net.ParseCIDR(s)
  if err != nil { panic(err) }
  return n
}

func RequirePrivateIP(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil { host = r.RemoteAddr }
    ip := net.ParseIP(host)
    for _, n := range allowedNets {
      if n.Contains(ip) {
        next.ServeHTTP(w, r)
        return
      }
    }
    http.Error(w, "forbidden", http.StatusForbidden)
  })
}
```

- [ ] **Step 2: Verify the metrics route uses the middleware**

In `main.go`, the metrics route should be registered as:

```go
r.With(middleware.RequirePrivateIP).Get("/api/metrics", promhttp.Handler().ServeHTTP)
```

- [ ] **Step 3: Run the backend tests to confirm nothing is broken**

```bash
cd backend && go test -race -count=1 ./...
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/middleware/ip_allowlist.go backend/cmd/server/main.go
git commit -m "security: restrict /api/metrics to private IP ranges only"
```

---

## Task 8: End-to-End Production Simulation

- [ ] **Step 1: Create a `.env` from `.env.example`**

```bash
cp .env.example .env
# Edit .env: fill in a test POSTGRES_PASSWORD, FRONTEND_URL=http://localhost:3000,
# SEED_ADMIN_EMAIL=admin@test.local, SMTP_HOST=mailpit, SMTP_PORT=1025,
# SMTP_USERNAME='', SMTP_PASSWORD='', SMTP_FROM=noreply@test.local
# RUSTFS_* values pointing to your local RustFS or a test bucket
```

- [ ] **Step 2: Start the full stack**

```bash
docker compose up --build -d
```

Expected: all containers start healthy.

- [ ] **Step 3: Verify health endpoints**

```bash
curl -s http://localhost:8080/api/health | jq .
```

Expected:

```json
{ "status": "ok" }
```

```bash
curl -s http://localhost:8080/api/health/ready | jq .
```

Expected:

```json
{ "status": "ok", "checks": { "database": "ok" } }
```

- [ ] **Step 4: Verify metrics is blocked externally**

From the host (outside Docker network), metrics should be accessible at localhost (Docker bridges localhost traffic):

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/metrics
```

Expected in production: `403` (if the reverse proxy forwards external traffic) or the endpoint must not be exposed via the reverse proxy config. Document the reverse proxy IP-restriction setup needed.

- [ ] **Step 5: Verify first-boot admin seeding**

```bash
docker compose logs backend | grep "seed_admin"
```

Expected log line:

```json
{ "level": "INFO", "msg": "seed_admin_created", "email": "admin@test.local" }
```

Check Mailpit for the magic link email:

```bash
open http://localhost:8025
```

- [ ] **Step 6: Full user journey test**

1. Click the magic link for the admin → verify redirect to `/`
2. Create a room → play a full round
3. Verify audit log in `/admin`
4. Delete a test user → verify hard-delete log and sentinel UUID in DB

- [ ] **Step 7: Run the production readiness script**

```bash
bash scripts/check-production-readiness.sh
```

Expected: all automated checks pass.

- [ ] **Step 8: Commit any fixes**

```bash
git commit -am "fix(ci): resolve production simulation issues"
```

---

## Manual Pre-Launch Items (Not Automatable)

These items must be completed by the operator before inviting users. They cannot be automated in CI:

- [ ] Reverse proxy configured to route `/api/*` to backend port 8080 and `/*` to frontend port 3000
- [ ] TLS certificate in place on reverse proxy (HTTPS is required for magic links and `Secure` cookies)
- [ ] Prometheus scrape target configured (optional — for monitoring)
- [ ] `GET /api/metrics` confirmed unreachable from the public internet via reverse proxy config
- [ ] PostgreSQL restore procedure tested manually: `gunzip backup.sql.gz | docker compose exec -T postgres psql -U fabyoumeme fabyoumeme`
- [ ] RustFS backup strategy confirmed with the RustFS stack operator
- [ ] SMTP provider Data Processing Agreement (DPA) signed — required per GDPR Art. 28
- [ ] Privacy Policy at `/privacy` reviewed and operator contact details filled in
- [ ] SEED_ADMIN_EMAIL first-boot email received and admin login verified
- [ ] Docker log driver verified on production host: `docker info | grep "Logging Driver"`
