ALTER TABLE knowledge_sources
  ADD COLUMN IF NOT EXISTS sync_error text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS last_sync_stats jsonb NOT NULL DEFAULT '{}';

CREATE UNIQUE INDEX IF NOT EXISTS documents_source_remote_token_idx
  ON documents(source_id, remote_token)
  WHERE source_id IS NOT NULL AND remote_token <> '';
