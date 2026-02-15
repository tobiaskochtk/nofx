#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"

echo "Starting local Redis + derivs ingest stack..."
docker compose up -d redis derivs-ingest

echo "Redis + derivs-ingest are running. Tail logs with:"
echo "  docker compose logs -f redis derivs-ingest"
