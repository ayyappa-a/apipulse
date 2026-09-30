#!/usr/bin/env bash
# run_api_local.sh — Run APIPulse API directly against the Compose infrastructure.
#
# Prerequisites:
#   docker compose up timescaledb redis vendor-mock -d
#
# This script connects to the TimescaleDB running in Docker on the
# HOST port 5433 (which maps to internal container port 5432).
# Use this ONLY when running `go run` directly — NOT inside a container.
#
# From the NEC root:
#   bash scripts/run_api_local.sh

set -euo pipefail

cd "$(dirname "$0")/.."

echo "→ Running APIPulse API locally against Compose infrastructure"
echo "  DATABASE_URL : postgres://apipulse:***@localhost:5433/apipulse"
echo "  REDIS_URL    : redis://localhost:6379"
echo "  VENDOR_MOCK  : http://localhost:9001"
echo ""

DATABASE_URL="postgres://apipulse:apipulse_local_dev@localhost:5433/apipulse?sslmode=disable" \
REDIS_URL="redis://localhost:6379" \
VENDOR_MOCK_URL="http://localhost:9001" \
MIGRATIONS_DIR="./backend/migrations" \
APIPULSE_STORAGE="postgres" \
APIPULSE_ENV="development" \
LOG_LEVEL="debug" \
  go run ./backend/cmd/api
