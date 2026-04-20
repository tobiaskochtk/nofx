#!/bin/sh
set -eu

is_true() {
  case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

start_embedded_redis() {
  REDIS_PORT="${EMBEDDED_REDIS_PORT:-6379}"
  REDIS_BIND="${EMBEDDED_REDIS_BIND:-127.0.0.1}"
  REDIS_DATA_DIR="${EMBEDDED_REDIS_DATA_DIR:-/app/data/redis}"

  mkdir -p "$REDIS_DATA_DIR"

  REDIS_PASS="${DERIVS_REDIS_PASSWORD:-}"

  if [ -n "$REDIS_PASS" ]; then
    redis-server \
      --bind "$REDIS_BIND" \
      --port "$REDIS_PORT" \
      --dir "$REDIS_DATA_DIR" \
      --appendonly yes \
      --requirepass "$REDIS_PASS" \
      >/app/data/embedded-redis.log 2>&1 &
  else
    redis-server \
      --bind "$REDIS_BIND" \
      --port "$REDIS_PORT" \
      --dir "$REDIS_DATA_DIR" \
      --appendonly yes \
      >/app/data/embedded-redis.log 2>&1 &
  fi

  REDIS_PID=$!
  echo "Embedded Redis started (pid=${REDIS_PID}, addr=${REDIS_BIND}:${REDIS_PORT})"

  # Block briefly until Redis is reachable to reduce startup races.
  i=0
  while [ "$i" -lt 30 ]; do
    if [ -n "$REDIS_PASS" ]; then
      if redis-cli -h "$REDIS_BIND" -p "$REDIS_PORT" -a "$REDIS_PASS" ping >/dev/null 2>&1; then
        break
      fi
    else
      if redis-cli -h "$REDIS_BIND" -p "$REDIS_PORT" ping >/dev/null 2>&1; then
        break
      fi
    fi
    i=$((i + 1))
    sleep 0.2
  done
  if [ "$i" -ge 30 ]; then
    echo "WARNING: Embedded Redis did not become ready before nofx startup"
  else
    echo "Embedded Redis is ready"
  fi

  export DERIVS_REDIS_ADDR="${DERIVS_REDIS_ADDR:-127.0.0.1:${REDIS_PORT}}"
  export DERIVS_REDIS_DB="${DERIVS_REDIS_DB:-0}"
  export DERIVS_REDIS_KEYSPACE="${DERIVS_REDIS_KEYSPACE:-derivs:v1}"
}

if is_true "${EMBEDDED_REDIS_ENABLED:-true}"; then
  start_embedded_redis
fi

exec /app/nofx
