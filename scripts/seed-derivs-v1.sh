#!/usr/bin/env bash
set -euo pipefail

if ! command -v redis-cli >/dev/null 2>&1; then
  echo "redis-cli is required for seeding" >&2
  exit 1
fi

REDIS_ADDR=${REDIS_ADDR:-127.0.0.1:6379}
REDIS_DB=${REDIS_DB:-0}
KEYSPACE=${KEYSPACE:-derivs:v1}

HOST=${REDIS_ADDR%%:*}
PORT=${REDIS_ADDR##*:}

function seed_key() {
  local type=$1
  local symbol=$2
  local payload=$3
  local key="${KEYSPACE}:${type}:${symbol}"
  redis-cli -h "$HOST" -p "$PORT" -n "$REDIS_DB" SET "$key" "$payload" >/dev/null
  echo "Seeded $key"
}

seed_key "oi" "LINKUSDT" '{
  "symbol": "LINK/USDT:USDT",
  "updated_at": 0,
  "samples": {
    "binanceusdm": [
      {"ts": 1700000000000, "value": 120000000},
      {"ts": 1700003600000, "value": 125000000}
    ]
  },
  "status": {"binanceusdm": "ok"}
}'

seed_key "funding" "LINKUSDT" '{
  "symbol": "LINK/USDT:USDT",
  "updated_at": 0,
  "samples": {
    "binanceusdm": [
      {"ts": 1700000000000, "rate": 0.0001},
      {"ts": 1700002880000, "rate": 0.00009}
    ]
  },
  "status": {"binanceusdm": "ok"}
}'

seed_key "basis" "LINKUSDT" '{
  "symbol": "LINK/USDT:USDT",
  "updated_at": 0,
  "samples": {
    "binanceusdm": [
      {"ts": 1700000000000, "mark": 13.1, "index": 13.0},
      {"ts": 1700003600000, "mark": 13.2, "index": 13.05}
    ]
  },
  "status": {"binanceusdm": "ok"}
}'
