#!/bin/sh
set -eu
timestamp="$(date +%Y%m%d-%H%M%S)"
target="${1:-./backups/$timestamp}"
mkdir -p "$target"
docker compose exec -T postgres pg_dump -U ai_agent -d ai_agent -Fc > "$target/postgres.dump"
docker compose exec -T minio sh -c 'tar -C /data -czf - .' > "$target/minio.tar.gz"
cp .env.example "$target/env.template"
echo "Backup created at $target"

