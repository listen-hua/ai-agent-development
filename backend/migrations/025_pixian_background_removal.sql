CREATE TABLE IF NOT EXISTS pixian_background_removal_config (
  id boolean PRIMARY KEY DEFAULT true CHECK (id),
  enabled boolean NOT NULL DEFAULT false,
  test_mode boolean NOT NULL DEFAULT true,
  encrypted_api_id text NOT NULL DEFAULT '',
  encrypted_api_secret text NOT NULL DEFAULT '',
  api_id_hint text NOT NULL DEFAULT '',
  api_secret_hint text NOT NULL DEFAULT '',
  timeout_seconds integer NOT NULL DEFAULT 180 CHECK (timeout_seconds BETWEEN 180 AND 600),
  concurrency integer NOT NULL DEFAULT 2 CHECK (concurrency BETWEEN 1 AND 5),
  max_pixels integer NOT NULL DEFAULT 25000000 CHECK (max_pixels BETWEEN 100 AND 25000000),
  account_state text NOT NULL DEFAULT '',
  account_credits double precision NOT NULL DEFAULT 0,
  account_checked_at timestamptz,
  updated_by uuid REFERENCES users(id),
  updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO pixian_background_removal_config(id) VALUES(true) ON CONFLICT(id) DO NOTHING;

CREATE TABLE IF NOT EXISTS background_removal_jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  canvas_id uuid REFERENCES image_canvases(id) ON DELETE SET NULL,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','retry','partial','succeeded','failed','cancelled')),
  test_mode boolean NOT NULL DEFAULT true,
  attempts integer NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  locked_until timestamptz,
  completed_count integer NOT NULL DEFAULT 0,
  failed_count integer NOT NULL DEFAULT 0,
  credits_charged double precision NOT NULL DEFAULT 0,
  credits_calculated double precision NOT NULL DEFAULT 0,
  error text NOT NULL DEFAULT '',
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(user_id,idempotency_key)
);
CREATE INDEX IF NOT EXISTS background_removal_jobs_claim_idx ON background_removal_jobs(status,next_attempt_at,locked_until);

ALTER TABLE image_canvas_nodes ADD COLUMN IF NOT EXISTS background_removal_job_id uuid REFERENCES background_removal_jobs(id) ON DELETE SET NULL;
ALTER TABLE image_canvas_nodes ADD COLUMN IF NOT EXISTS source_node_id uuid REFERENCES image_canvas_nodes(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS background_removal_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id uuid NOT NULL REFERENCES background_removal_jobs(id) ON DELETE CASCADE,
  source_node_id uuid REFERENCES image_canvas_nodes(id) ON DELETE SET NULL,
  source_asset_id uuid NOT NULL REFERENCES image_assets(id) ON DELETE RESTRICT,
  project_id uuid NOT NULL REFERENCES image_projects(id) ON DELETE RESTRICT,
  placeholder_node_id uuid REFERENCES image_canvas_nodes(id) ON DELETE SET NULL,
  result_asset_id uuid REFERENCES image_assets(id),
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','succeeded','failed')),
  attempts integer NOT NULL DEFAULT 0,
  credits_charged double precision NOT NULL DEFAULT 0,
  credits_calculated double precision NOT NULL DEFAULT 0,
  input_size text NOT NULL DEFAULT '',
  result_size text NOT NULL DEFAULT '',
  error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(job_id,source_node_id)
);

-- Keep source/result history even when a user removes either canvas node. These
-- statements are intentionally idempotent so an earlier development version of
-- this migration (which used RESTRICT/CASCADE) is corrected on existing stacks.
ALTER TABLE background_removal_items
    DROP CONSTRAINT IF EXISTS background_removal_items_source_node_id_fkey;
ALTER TABLE background_removal_items
    ADD CONSTRAINT background_removal_items_source_node_id_fkey
    FOREIGN KEY (source_node_id) REFERENCES image_canvas_nodes(id) ON DELETE SET NULL;
ALTER TABLE background_removal_items
    DROP CONSTRAINT IF EXISTS background_removal_items_placeholder_node_id_fkey;
ALTER TABLE background_removal_items
    ADD CONSTRAINT background_removal_items_placeholder_node_id_fkey
    FOREIGN KEY (placeholder_node_id) REFERENCES image_canvas_nodes(id) ON DELETE SET NULL;
CREATE UNIQUE INDEX IF NOT EXISTS background_removal_active_source_idx
  ON background_removal_items(source_node_id) WHERE status IN ('pending','running');

ALTER TABLE image_assets ADD COLUMN IF NOT EXISTS source_asset_id uuid REFERENCES image_assets(id) ON DELETE SET NULL;
ALTER TABLE image_assets DROP CONSTRAINT IF EXISTS image_assets_source_check;
ALTER TABLE image_assets ADD CONSTRAINT image_assets_source_check CHECK (source IN ('upload','generated','background_removed'));
