# 10 — Observability

## Structured Logging

The backend uses Go's `log/slog` (stdlib, no external dependency) for structured JSON logs. Every log entry follows this schema:

```json
{
  "time": "2026-03-27T12:34:56.789Z",
  "level": "INFO",
  "msg": "auth_verify_success",
  "service": "backend",
  "request_id": "req_01jq3abc...",
  "user_id": "uuid",
  "duration_ms": 12,
  "error": null
}
```

`request_id` is a random ID generated in the logging middleware and added to the request context. It is also returned as the `X-Request-ID` response header so frontend error reports can be correlated with backend logs.

### Key Log Events

| `msg` value               | Level | When                                                                                 |
| ------------------------- | ----- | ------------------------------------------------------------------------------------ |
| `auth_register_success`   | INFO  | User created via invite                                                              |
| `auth_magic_link_sent`    | INFO  | Magic link email dispatched                                                          |
| `auth_verify_success`     | INFO  | Token verified, session created                                                      |
| `auth_verify_failure`     | WARN  | Invalid / expired / used token                                                       |
| `auth_logout`             | INFO  | Session deleted                                                                      |
| `session_created`         | INFO  | Session row inserted                                                                 |
| `session_expired`         | INFO  | Session lookup rejected (expired)                                                    |
| `user_deactivated`        | WARN  | `is_active` set to false by admin                                                    |
| `admin_action`            | INFO  | Any admin route that modifies data — includes `action`, `resource`, `changes` fields |
| `room_created`            | INFO  | Room row inserted                                                                    |
| `room_started`            | INFO  | Game state transition `lobby → playing`                                              |
| `room_finished`           | INFO  | Game state transition `playing → finished`                                           |
| `round_started`           | INFO  | Round row inserted + `round_started` event broadcast                                 |
| `round_ended`             | INFO  | `submissions_closed` broadcast                                                       |
| `vote_results_broadcast`  | INFO  | `vote_results` event sent                                                            |
| `game_ended`              | INFO  | `game_ended` event broadcast + reason                                                |
| `ws_connect`              | INFO  | WebSocket upgrade accepted                                                           |
| `ws_disconnect`           | INFO  | WebSocket closed (includes reason)                                                   |
| `ws_rate_limited`         | WARN  | Connection dropped for exceeding 20 msg/s                                            |
| `ws_reconnect`            | INFO  | Player reconnected within grace window                                               |
| `ws_grace_expired`        | WARN  | Player's grace window expired, removed from room                                     |
| `asset_upload_url_issued` | INFO  | Pre-signed upload URL generated                                                      |
| `asset_upload_confirmed`  | INFO  | `media_key` stored on item                                                           |
| `email_send_failure`      | ERROR | SMTP delivery failed                                                                 |
| `db_query_slow`           | WARN  | Query exceeded 500ms                                                                 |
| `startup`                 | INFO  | Server ready; includes version, port, env                                            |
| `shutdown`                | INFO  | Graceful shutdown initiated                                                          |

---

## Health Endpoints

### `GET /api/health`

Liveness check. Returns immediately — indicates only that the process is running.

```json
200 OK
{ "status": "ok" }
```

### `GET /api/health/deep`

Readiness check. Performs active dependency checks before responding.

Checks:

1. **PostgreSQL**: `SELECT 1` with a 2-second timeout
2. **RustFS**: HEAD request to the bucket root with a 2-second timeout

```json
200 OK
{
  "status": "ok",
  "checks": {
    "postgres": "ok",
    "rustfs": "ok"
  }
}
```

```json
503 Service Unavailable
{
  "status": "degraded",
  "checks": {
    "postgres": "ok",
    "rustfs": "error: connection refused"
  }
}
```

Use `GET /api/health` for container liveness probes (fast). Use `GET /api/health/deep` for readiness probes after startup.

---

## Prometheus Metrics

Exposed at `GET /api/metrics` in Prometheus text format. **Bind this to a non-public port or IP-restrict it** — it must never be reachable from the internet.

### HTTP Metrics

```plain
http_requests_total{method, status_class, path_pattern}
  Counter. path_pattern uses templated paths (e.g. "/api/rooms/{code}") to avoid cardinality explosion.

http_request_duration_seconds{method, path_pattern}
  Histogram. Buckets: 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5
```

### WebSocket Metrics

```plain
websocket_connections_active
  Gauge. Current open WebSocket connections.

websocket_messages_total{direction, type}
  Counter. direction = "client_to_server" | "server_to_client"

websocket_reconnects_total
  Counter. Successful reconnections within grace window.

websocket_grace_expirations_total
  Counter. Players whose grace window expired.
```

### Game Metrics

```plain
game_rooms_total{state}
  Gauge. Rooms grouped by current state (lobby, playing, finished).

game_rounds_total{game_type, outcome}
  Counter. outcome = "completed" | "skipped" (e.g. all players disconnected mid-round)

game_submissions_total{game_type}
  Counter.

game_votes_total{game_type}
  Counter.
```

### Auth Metrics

```plain
magic_links_sent_total
  Counter.

magic_links_verified_total{result}
  Counter. result = "success" | "expired" | "used" | "not_found"

sessions_created_total
  Counter.

sessions_active
  Gauge. Sessions with expires_at > now() (sampled, not exact count).
```

### Infrastructure Metrics

```plain
db_query_duration_seconds{query_name}
  Histogram. Labelled with sqlc query name.

db_connections_active
  Gauge. PostgreSQL connection pool in-use count.

email_sends_total{result}
  Counter. result = "success" | "failure"
```

---

## Log Retention

Logs are written to stdout/stderr and captured by Docker. Recommended log rotation policy on the host:

```json
// /etc/docker/daemon.json
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "50m",
    "max-file": "10"
  }
}
```

This retains up to 500 MB of compressed logs per container. Adjust based on available disk space.
