CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  feishu_open_id text UNIQUE NOT NULL,
  name text NOT NULL,
  avatar_url text NOT NULL DEFAULT '',
  department_ids text[] NOT NULL DEFAULT '{}',
  job_title text NOT NULL DEFAULT '',
  job_level_id text NOT NULL DEFAULT '',
  job_family_id text NOT NULL DEFAULT '',
  employee_type integer NOT NULL DEFAULT 0,
  status text NOT NULL DEFAULT 'active',
  organization_synced_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS role_assignments (
  user_id uuid NOT NULL REFERENCES users(id),
  role text NOT NULL CHECK (role IN ('employee','knowledge_admin','notification_admin','auditor','super_admin')),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, role)
);

CREATE TABLE IF NOT EXISTS knowledge_sources (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  type text NOT NULL CHECK (type IN ('upload','feishu_folder','feishu_wiki')),
  remote_token text NOT NULL DEFAULT '',
  default_acl jsonb NOT NULL DEFAULT '{"scope":"all"}',
  sync_status text NOT NULL DEFAULT 'idle',
  last_synced_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS documents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id uuid REFERENCES knowledge_sources(id),
  title text NOT NULL,
  source_url text NOT NULL DEFAULT '',
  remote_token text NOT NULL DEFAULT '',
  acl jsonb NOT NULL DEFAULT '{"scope":"all"}',
  status text NOT NULL DEFAULT 'draft',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS document_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id uuid NOT NULL REFERENCES documents(id),
  version text NOT NULL,
  checksum text NOT NULL,
  object_key text NOT NULL DEFAULT '',
  mime_type text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'draft',
  effective_at timestamptz,
  expires_at timestamptz,
  published_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(document_id, version)
);

CREATE TABLE IF NOT EXISTS chunks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id uuid NOT NULL REFERENCES documents(id),
  version_id uuid NOT NULL REFERENCES document_versions(id),
  ordinal integer NOT NULL,
  heading text NOT NULL DEFAULT '',
  page integer NOT NULL DEFAULT 0,
  content text NOT NULL,
  search_vector tsvector GENERATED ALWAYS AS (to_tsvector('simple', content)) STORED,
  embedding vector(1024),
  metadata jsonb NOT NULL DEFAULT '{}',
  UNIQUE(version_id, ordinal)
);
CREATE INDEX IF NOT EXISTS chunks_search_idx ON chunks USING gin(search_vector);
CREATE INDEX IF NOT EXISTS chunks_embedding_idx ON chunks USING hnsw (embedding vector_cosine_ops);

CREATE TABLE IF NOT EXISTS conversations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  title text NOT NULL DEFAULT '新会话',
  deleted_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  conversation_id uuid NOT NULL REFERENCES conversations(id),
  role text NOT NULL CHECK (role IN ('user','assistant')),
  content text NOT NULL,
  citations jsonb NOT NULL DEFAULT '[]',
  model text NOT NULL DEFAULT '',
  prompt_tokens integer NOT NULL DEFAULT 0,
  completion_tokens integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS agent_config_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  version integer UNIQUE NOT NULL,
  status text NOT NULL CHECK (status IN ('draft','published','archived')),
  config jsonb NOT NULL,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz
);

CREATE TABLE IF NOT EXISTS notification_drafts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  title text NOT NULL,
  content text NOT NULL,
  audience jsonb NOT NULL,
  status text NOT NULL DEFAULT 'draft',
  scheduled_at timestamptz,
  approved_by uuid REFERENCES users(id),
  created_by uuid NOT NULL REFERENCES users(id),
  idempotency_key text UNIQUE NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS delivery_attempts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  notification_id uuid NOT NULL REFERENCES notification_drafts(id),
  receiver_open_id text NOT NULL,
  status text NOT NULL,
  message_id text NOT NULL DEFAULT '',
  error text NOT NULL DEFAULT '',
  attempts integer NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(notification_id, receiver_open_id)
);

CREATE TABLE IF NOT EXISTS audit_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id uuid REFERENCES users(id),
  action text NOT NULL,
  resource_type text NOT NULL,
  resource_id text NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS processed_events (
  event_id text PRIMARY KEY,
  received_at timestamptz NOT NULL DEFAULT now()
);
