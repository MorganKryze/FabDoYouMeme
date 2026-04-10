#!/usr/bin/env bash
# scripts/backup.sh
# Run nightly via host cron: 0 2 * * * /path/to/FabDoYouMeme/scripts/backup.sh
# Requires: BACKUP_DIR env var (or defaults to /var/backups/fabyoumeme)
#           COMPOSE_FILE env var pointing to docker compose base file location

set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-$(dirname "$0")/../docker/compose.base.yml}"
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
