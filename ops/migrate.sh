#!/bin/sh
set -eu

for file in /migrations/*.sql; do
  psql -v ON_ERROR_STOP=1 -h postgres -U ai_agent -d ai_agent -f "$file"
done
