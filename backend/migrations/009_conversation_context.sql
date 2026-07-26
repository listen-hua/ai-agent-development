ALTER TABLE conversations
  ADD COLUMN IF NOT EXISTS agent_key text NOT NULL DEFAULT 'administrative_assistant',
  ADD COLUMN IF NOT EXISTS channel text NOT NULL DEFAULT 'h5';

ALTER TABLE messages
  ADD COLUMN IF NOT EXISTS intent text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS standalone_query text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS context_version integer NOT NULL DEFAULT 0;

UPDATE agent_config_versions
SET config = config || jsonb_build_object('context_model', 'qwen-flash')
WHERE NOT (config ? 'context_model');

CREATE TABLE IF NOT EXISTS conversation_contexts (
  conversation_id uuid PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
  summary text NOT NULL DEFAULT '',
  summarized_through_message_id uuid REFERENCES messages(id) ON DELETE SET NULL,
  reset_through_message_id uuid REFERENCES messages(id) ON DELETE SET NULL,
  active_task jsonb NOT NULL DEFAULT '{}',
  version integer NOT NULL DEFAULT 0,
  expires_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE conversation_contexts
  ADD COLUMN IF NOT EXISTS reset_through_message_id uuid REFERENCES messages(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS conversation_bindings (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  agent_key text NOT NULL,
  channel text NOT NULL,
  external_scope_id text NOT NULL,
  conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(user_id, agent_key, channel, external_scope_id)
);

CREATE INDEX IF NOT EXISTS conversation_bindings_expiry_idx
  ON conversation_bindings(expires_at);

CREATE INDEX IF NOT EXISTS messages_conversation_created_idx
  ON messages(conversation_id, created_at, id);
