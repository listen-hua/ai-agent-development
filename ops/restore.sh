#!/bin/sh
set -eu
source_dir="${1:?usage: restore.sh <backup-directory>}"
test -f "$source_dir/postgres.dump"
test -f "$source_dir/minio.tar.gz"
docker compose exec -T postgres pg_restore -U ai_agent -d ai_agent --clean --if-exists < "$source_dir/postgres.dump"
docker compose exec -T minio sh -c 'rm -rf /data/* && tar -C /data -xzf -' < "$source_dir/minio.tar.gz"
echo "Restore complete; restart api and worker after verification."
