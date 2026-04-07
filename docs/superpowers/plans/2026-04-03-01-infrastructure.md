# Infrastructure & Project Bootstrap — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the complete project scaffold — Go module, SvelteKit project, Docker Compose stack, and `.env.example` — so every subsequent phase has a runnable base to build on.

**Architecture:** Go backend at `backend/`, SvelteKit frontend at `frontend/`, shared via Docker Compose. No application logic here — only the shell that later phases fill in.

**Tech Stack:** Go 1.23, SvelteKit + Svelte 5, Tailwind CSS v4, shadcn-svelte, Docker Compose, golang-migrate CLI.

---

### Task 1: Go module + minimal server entrypoint

**Files:**

- Create: `backend/go.mod`
- Create: `backend/go.sum` (generated)
- Create: `backend/cmd/server/main.go`

- [ ] **Step 1: Initialise Go module**

```bash
mkdir -p backend/cmd/server backend/internal backend/db/migrations backend/db/queries
cd backend
go mod init github.com/MorganKryze/FabDoYouMeme/backend
```

- [ ] **Step 2: Add all required dependencies**

```bash
cd backend
go get github.com/go-chi/chi/v5@v5.1.0
go get github.com/gorilla/websocket@v1.5.3
go get github.com/google/uuid@v1.6.0
go get github.com/jackc/pgx/v5@v5.6.0
go get github.com/aws/aws-sdk-go-v2@v1.30.0
go get github.com/aws/aws-sdk-go-v2/config@v1.27.0
go get github.com/aws/aws-sdk-go-v2/service/s3@v1.57.0
go get github.com/aws/aws-sdk-go-v2/credentials@v1.17.0
go get github.com/wneessen/go-mail@v0.4.2
go get golang.org/x/crypto@v0.24.0
go get github.com/sqlc-dev/sqlc@v1.26.0
go get -tool github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1
```

- [ ] **Step 3: Write minimal `main.go` that compiles and prints startup message**

```go
// backend/cmd/server/main.go
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	slog.Info("startup", "port", port)
	fmt.Printf("listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Verify it compiles**

```bash
cd backend && go build ./...
```

Expected: no output (success).

- [ ] **Step 5: Write minimal Dockerfile**

```dockerfile
# backend/Dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/server .
COPY internal/email/templates ./internal/email/templates
EXPOSE 8080
CMD ["./server"]
```

---

### Task 2: SvelteKit project bootstrap

**Files:**

- Create: `frontend/` (SvelteKit project)
- Create: `frontend/Dockerfile`

- [ ] **Step 1: Scaffold SvelteKit project**

```bash
cd frontend
npm create svelte@latest . -- --template skeleton --types ts --no-prettier --no-eslint --no-playwright --no-vitest
```

When prompted: select `Skeleton project`, TypeScript, no extras.

- [ ] **Step 2: Install dependencies**

```bash
cd frontend
npm install
npm install -D tailwindcss@next @tailwindcss/vite
npm install -D @shadcn-svelte/cli
npm install lucide-svelte
```

- [ ] **Step 3: Configure Tailwind v4**

Create `frontend/src/app.css`:

```css
@import 'tailwindcss';
```

Update `frontend/src/app.html` to include the stylesheet:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <link rel="icon" href="%sveltekit.assets%/favicon.png" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    %sveltekit.head%
  </head>
  <body data-sveltekit-preload-data="hover">
    <div style="display: contents">%sveltekit.body%</div>
  </body>
</html>
```

Update `frontend/vite.config.ts`:

```ts
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [tailwindcss(), sveltekit()]
});
```

- [ ] **Step 4: Configure `adapter-node`**

```bash
cd frontend && npm install -D @sveltejs/adapter-node
```

Update `frontend/svelte.config.js`:

```js
import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter(),
    csp: {
      mode: 'nonce',
      directives: {
        'default-src': ['self'],
        'script-src': ['self'],
        'style-src': ['self'],
        'font-src': ['self'],
        'img-src': ['self', 'data:', 'blob:'],
        'connect-src': ['self', 'wss:', 'ws:'],
        'frame-ancestors': ['none']
      }
    }
  }
};

export default config;
```

- [ ] **Step 5: Write frontend Dockerfile**

```dockerfile
# frontend/Dockerfile
FROM node:22-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:22-alpine
WORKDIR /app
COPY --from=builder /app/build ./build
COPY --from=builder /app/package.json .
ENV NODE_ENV=production
EXPOSE 3000
CMD ["node", "build"]
```

- [ ] **Step 6: Verify frontend builds**

```bash
cd frontend && PUBLIC_API_URL=http://localhost:8080 npm run build
```

Expected: `build/` directory created, no errors.

---

### Task 3: Docker Compose + environment files

**Files:**

- Create: `docker-compose.yml`
- Create: `docker-compose.override.yml`
- Create: `.env.example`
- Create: `.gitignore` (update)

- [ ] **Step 1: Write `docker-compose.yml`**

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:17-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: fabyoumeme
      POSTGRES_USER: fabyoumeme
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ['CMD-SHELL', 'pg_isready -U fabyoumeme']
      interval: 5s
      retries: 5
    expose:
      - 5432
    networks:
      - project_network

  backend:
    build: ./backend
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://fabyoumeme:${POSTGRES_PASSWORD}@postgres:5432/fabyoumeme
      RUSTFS_ENDPOINT: ${RUSTFS_ENDPOINT}
      RUSTFS_ACCESS_KEY: ${RUSTFS_ACCESS_KEY}
      RUSTFS_SECRET_KEY: ${RUSTFS_SECRET_KEY}
      RUSTFS_BUCKET: ${RUSTFS_BUCKET:-fabyoumeme-assets}
      ALLOWED_ORIGIN: ${FRONTEND_URL}
      SMTP_HOST: ${SMTP_HOST}
      SMTP_PORT: ${SMTP_PORT:-587}
      SMTP_USERNAME: ${SMTP_USERNAME}
      SMTP_PASSWORD: ${SMTP_PASSWORD}
      SMTP_FROM: ${SMTP_FROM}
      MAGIC_LINK_BASE_URL: ${FRONTEND_URL}
      MAGIC_LINK_TTL: ${MAGIC_LINK_TTL:-15m}
      SESSION_TTL: ${SESSION_TTL:-720h}
      SEED_ADMIN_EMAIL: ${SEED_ADMIN_EMAIL}
      PORT: ${BACKEND_PORT:-8080}
      LOG_LEVEL: ${LOG_LEVEL:-info}
      RECONNECT_GRACE_WINDOW: ${RECONNECT_GRACE_WINDOW:-30s}
      WS_RATE_LIMIT: ${WS_RATE_LIMIT:-20}
      WS_READ_LIMIT_BYTES: ${WS_READ_LIMIT_BYTES:-4096}
      WS_READ_DEADLINE: ${WS_READ_DEADLINE:-60s}
      WS_PING_INTERVAL: ${WS_PING_INTERVAL:-25s}
      SESSION_RENEW_INTERVAL: ${SESSION_RENEW_INTERVAL:-60m}
      MAX_UPLOAD_SIZE_BYTES: ${MAX_UPLOAD_SIZE_BYTES:-2097152}
      RATE_LIMIT_AUTH_RPM: ${RATE_LIMIT_AUTH_RPM:-10}
      RATE_LIMIT_INVITE_VALIDATION_RPH: ${RATE_LIMIT_INVITE_VALIDATION_RPH:-20}
      RATE_LIMIT_ROOMS_RPH: ${RATE_LIMIT_ROOMS_RPH:-10}
      RATE_LIMIT_UPLOADS_RPH: ${RATE_LIMIT_UPLOADS_RPH:-50}
      RATE_LIMIT_GLOBAL_RPM: ${RATE_LIMIT_GLOBAL_RPM:-100}
    expose:
      - 8080
    networks:
      - project_network

  frontend:
    build: ./frontend
    restart: unless-stopped
    depends_on:
      - backend
    environment:
      PUBLIC_API_URL: ${BACKEND_URL}
    expose:
      - 3000
    networks:
      - project_network
      - pangolin

volumes:
  postgres_data:

networks:
  project_network:
    driver: bridge
  pangolin:
    external: true
```

- [ ] **Step 2: Write `docker-compose.override.yml`**

```yaml
# docker-compose.override.yml  — dev only
services:
  mailpit:
    image: axllent/mailpit:latest
    ports:
      - '8025:8025'
    expose:
      - 1025
    networks:
      - project_network

  backend:
    environment:
      SMTP_HOST: mailpit
      SMTP_PORT: '1025'
      SMTP_USERNAME: ''
      SMTP_PASSWORD: ''
      SMTP_FROM: noreply@fabyoumeme.local
      LOG_LEVEL: debug
    volumes:
      - ./backend:/app

  frontend:
    volumes:
      - ./frontend:/app

networks:
  project_network:
    external: true
```

- [ ] **Step 3: Write `.env.example`**

```bash
# .env.example — copy to .env and fill in before first boot

# Database
POSTGRES_PASSWORD=change_me_strong_password

# RustFS (deployed separately on pangolin network)
RUSTFS_ENDPOINT=https://rustfs.example.com
RUSTFS_ACCESS_KEY=your_access_key
RUSTFS_SECRET_KEY=your_secret_key
RUSTFS_BUCKET=fabyoumeme-assets

# URLs
FRONTEND_URL=https://meme.example.com
BACKEND_URL=https://meme.example.com/api

# SMTP (DPA required with provider before launch — see design/ref-gdpr.md)
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USERNAME=your_username
SMTP_PASSWORD=your_smtp_password
SMTP_FROM=noreply@example.com

# First-boot admin (set once; idempotent on restart)
SEED_ADMIN_EMAIL=admin@example.com

# Optional overrides (defaults shown)
# LOG_LEVEL=info
# SESSION_TTL=720h
# MAGIC_LINK_TTL=15m
# BACKEND_PORT=8080
# MAX_UPLOAD_SIZE_BYTES=2097152
# RECONNECT_GRACE_WINDOW=30s
```

- [ ] **Step 4: Update `.gitignore`**

```gitignore
# .gitignore
.env
*.env.local
*.env.production

# Go
backend/vendor/
backend/tmp/

# Node
frontend/node_modules/
frontend/.svelte-kit/
frontend/build/

# IDE
.idea/
.vscode/settings.json
*.swp

# OS
.DS_Store
Thumbs.db

# Docker
postgres_data/
```

- [ ] **Step 5: Verify Docker Compose config is valid**

```bash
docker compose config --quiet
```

Expected: no errors. (Will warn about `pangolin` network not existing — that's fine for dev.)

---

### Task 4: sqlc configuration

**Files:**

- Create: `backend/sqlc.yaml`

- [ ] **Step 1: Write `sqlc.yaml`**

```yaml
# backend/sqlc.yaml
version: '2'
sql:
  - engine: 'postgresql'
    queries: 'db/queries/'
    schema: 'db/migrations/'
    gen:
      go:
        package: 'db'
        out: 'db/sqlc'
        sql_package: 'pgx/v5'
        emit_json_tags: true
        emit_pointers_for_null_types: true
        overrides:
          - db_type: 'uuid'
            go_type: 'github.com/google/uuid.UUID'
          - db_type: 'timestamptz'
            go_type: 'time.Time'
          - db_type: 'jsonb'
            go_type: 'encoding/json.RawMessage'
```

- [ ] **Step 2: Verify sqlc is installed and config is valid**

```bash
cd backend && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.26.0 vet
```

Expected: "sqlc: no errors" (or "no queries found" since queries dir is empty — both are fine).

---

### Verification

```bash
cd backend && go build ./...   # must succeed
cd frontend && npm run check   # must succeed with 0 errors
docker compose config --quiet  # must succeed
```

Mark phase 1 complete in `docs/implementation-status.md`.
