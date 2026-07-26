ALTER TABLE role_assignments DROP CONSTRAINT IF EXISTS role_assignments_role_check;
ALTER TABLE role_assignments ADD CONSTRAINT role_assignments_role_check
  CHECK (role IN ('employee','knowledge_admin','notification_admin','image_admin','auditor','super_admin'));

CREATE TABLE IF NOT EXISTS image_relays (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  relay_key text UNIQUE NOT NULL,
  name text NOT NULL,
  base_url text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  timeout_seconds integer NOT NULL DEFAULT 120 CHECK (timeout_seconds BETWEEN 10 AND 600),
  allowed_output_hosts text[] NOT NULL DEFAULT '{}',
  encrypted_api_key text NOT NULL DEFAULT '',
  api_key_hint text NOT NULL DEFAULT '',
  created_by uuid REFERENCES users(id),
  updated_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS image_models (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  relay_id uuid NOT NULL REFERENCES image_relays(id) ON DELETE CASCADE,
  model_id text NOT NULL,
  display_name text NOT NULL,
  protocol text NOT NULL DEFAULT 'chat_completions'
    CHECK (protocol IN ('chat_completions','images_generations')),
  enabled boolean NOT NULL DEFAULT false,
  supports_reference boolean NOT NULL DEFAULT false,
  supports_reverse boolean NOT NULL DEFAULT true,
  supported_sizes text[] NOT NULL DEFAULT ARRAY['1K','2K','4K'],
  max_count integer NOT NULL DEFAULT 1 CHECK (max_count IN (1,2,4,8)),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(relay_id, model_id)
);

CREATE TABLE IF NOT EXISTS image_projects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_key text UNIQUE NOT NULL,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  acl jsonb NOT NULL DEFAULT '{"scope":"all"}',
  enabled boolean NOT NULL DEFAULT true,
  created_by uuid REFERENCES users(id),
  updated_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS image_prompt_actions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  action_key text NOT NULL,
  name text NOT NULL,
  prompt_template text NOT NULL,
  project_id uuid REFERENCES image_projects(id) ON DELETE CASCADE,
  enabled boolean NOT NULL DEFAULT true,
  sort_order integer NOT NULL DEFAULT 0,
  created_by uuid REFERENCES users(id),
  updated_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS image_prompt_actions_global_key
  ON image_prompt_actions(action_key) WHERE project_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS image_prompt_actions_project_key
  ON image_prompt_actions(project_id, action_key) WHERE project_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS image_canvases (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id uuid NOT NULL REFERENCES image_projects(id) ON DELETE CASCADE,
  viewport jsonb NOT NULL DEFAULT '{"x":0,"y":0,"zoom":1}',
  version bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(user_id, project_id)
);

CREATE TABLE IF NOT EXISTS image_assets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id uuid NOT NULL REFERENCES users(id),
  project_id uuid NOT NULL REFERENCES image_projects(id),
  object_key text UNIQUE NOT NULL,
  mime_type text NOT NULL CHECK (mime_type IN ('image/jpeg','image/png','image/webp')),
  file_name text NOT NULL DEFAULT '',
  width integer NOT NULL DEFAULT 0,
  height integer NOT NULL DEFAULT 0,
  size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
  source text NOT NULL CHECK (source IN ('upload','generated')),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS image_jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  project_id uuid NOT NULL REFERENCES image_projects(id),
  canvas_id uuid NOT NULL REFERENCES image_canvases(id),
  relay_id uuid NOT NULL REFERENCES image_relays(id),
  model_id uuid NOT NULL REFERENCES image_models(id),
  kind text NOT NULL CHECK (kind IN ('generate','reverse_prompt')),
  prompt text NOT NULL DEFAULT '',
  aspect_ratio text NOT NULL DEFAULT '1:1' CHECK (aspect_ratio IN ('1:1','16:9','9:16','4:3','3:4')),
  image_size text NOT NULL DEFAULT '1K' CHECK (image_size IN ('1K','2K','4K')),
  count integer NOT NULL DEFAULT 1 CHECK (count IN (1,2,4,8)),
  reference_asset_ids uuid[] NOT NULL DEFAULT '{}',
  status text NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','running','retry','partial','succeeded','failed','cancelled')),
  attempts integer NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  locked_until timestamptz,
  completed_count integer NOT NULL DEFAULT 0,
  reversed_prompt text NOT NULL DEFAULT '',
  error text NOT NULL DEFAULT '',
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(user_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS image_jobs_claim_idx ON image_jobs(status,next_attempt_at,locked_until);

CREATE TABLE IF NOT EXISTS image_canvas_nodes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  canvas_id uuid NOT NULL REFERENCES image_canvases(id) ON DELETE CASCADE,
  asset_id uuid REFERENCES image_assets(id),
  job_id uuid REFERENCES image_jobs(id) ON DELETE SET NULL,
  output_index integer NOT NULL DEFAULT 0,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','ready','failed')),
  x double precision NOT NULL,
  y double precision NOT NULL,
  width double precision NOT NULL DEFAULT 360,
  height double precision NOT NULL DEFAULT 360,
  z_index integer NOT NULL DEFAULT 1,
  error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS image_canvas_nodes_canvas_idx ON image_canvas_nodes(canvas_id,z_index,created_at);

CREATE TABLE IF NOT EXISTS image_job_outputs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id uuid NOT NULL REFERENCES image_jobs(id) ON DELETE CASCADE,
  output_index integer NOT NULL,
  asset_id uuid REFERENCES image_assets(id),
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','running','succeeded','failed')),
  error text NOT NULL DEFAULT '',
  attempts integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(job_id, output_index)
);

INSERT INTO image_relays(id,relay_key,name,base_url,enabled,timeout_seconds,allowed_output_hosts)
VALUES
  ('00000000-0000-4000-8000-000000000201','xgapi','XGAPI','https://api.xgapi.top/v1',true,120,ARRAY['api.xgapi.top']),
  ('00000000-0000-4000-8000-000000000202','comfly','Comfly AI','https://ai.comfly.org/v1',true,120,ARRAY['ai.comfly.org'])
ON CONFLICT(relay_key) DO UPDATE SET name=EXCLUDED.name,base_url=EXCLUDED.base_url;

INSERT INTO image_projects(id,project_key,name,description,acl,enabled)
VALUES ('00000000-0000-4000-8000-000000000301','general','通用创意','公司通用 AI 生图项目','{"scope":"all"}',true)
ON CONFLICT(project_key) DO NOTHING;

UPDATE image_projects
SET name='通用创意',
    description='公司通用 AI 生图项目',
    updated_at=now()
WHERE project_key='general'
  AND (name=repeat(chr(63),4) OR position(chr(63) in description)>0);
