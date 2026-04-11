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
check "docker/compose.base.yml valid" "docker compose -f docker/compose.base.yml config > /dev/null"
check "Log rotation configured" "grep -q 'max-size' docker/compose.base.yml"

echo ""
echo "──────────────────────────────────────────"
echo "Results: $PASS passed, $FAIL failed"
echo ""

if [ "$FAIL" -gt 0 ]; then
  echo "Fix the failing items above before going live."
  exit 1
else
  echo "All checks passed. Review the manual items in docs/operations.md before inviting users."
  exit 0
fi
