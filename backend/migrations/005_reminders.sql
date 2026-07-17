CREATE TABLE IF NOT EXISTS reminders (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  content text NOT NULL CHECK (char_length(content) BETWEEN 1 AND 500),
  status text NOT NULL CHECK (status IN ('active','paused','completed','cancelled','failed')),
  schedule_type text NOT NULL CHECK (schedule_type IN ('once','daily','workday','weekly')),
  timezone text NOT NULL DEFAULT 'Asia/Shanghai',
  local_time time,
  weekdays smallint[] NOT NULL DEFAULT '{}',
  once_at timestamptz,
  next_fire_at timestamptz,
  last_fired_at timestamptz,
  last_error text NOT NULL DEFAULT '',
  source_channel text NOT NULL DEFAULT 'h5' CHECK (source_channel IN ('h5','feishu_bot')),
  source_conversation_id uuid REFERENCES conversations(id) ON DELETE SET NULL,
  version integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (
    (schedule_type='once' AND once_at IS NOT NULL) OR
    (schedule_type IN ('daily','workday') AND local_time IS NOT NULL) OR
    (schedule_type='weekly' AND local_time IS NOT NULL AND cardinality(weekdays)>0)
  )
);
CREATE INDEX IF NOT EXISTS reminders_user_idx ON reminders(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS reminders_due_idx ON reminders(next_fire_at) WHERE status='active';

CREATE TABLE IF NOT EXISTS reminder_action_drafts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  action text NOT NULL CHECK (action IN ('create','update','pause','resume','delete')),
  reminder_id uuid REFERENCES reminders(id) ON DELETE CASCADE,
  content text NOT NULL DEFAULT '',
  schedule jsonb,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','confirmed','cancelled','expired')),
  source_channel text NOT NULL DEFAULT 'h5' CHECK (source_channel IN ('h5','feishu_bot')),
  source_conversation_id uuid REFERENCES conversations(id) ON DELETE SET NULL,
  result_reminder_id uuid REFERENCES reminders(id) ON DELETE SET NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  confirmed_at timestamptz
);
CREATE INDEX IF NOT EXISTS reminder_action_user_idx ON reminder_action_drafts(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS reminder_action_expiry_idx ON reminder_action_drafts(expires_at) WHERE status='pending';

ALTER TABLE messages ADD COLUMN IF NOT EXISTS reminder_action_id uuid REFERENCES reminder_action_drafts(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS reminder_deliveries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reminder_id uuid NOT NULL REFERENCES reminders(id) ON DELETE CASCADE,
  scheduled_for timestamptz NOT NULL,
  status text NOT NULL CHECK (status IN ('pending','sending','retry','sent','missed','failed')),
  attempts integer NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL,
  locked_until timestamptz,
  message_id text NOT NULL DEFAULT '',
  error text NOT NULL DEFAULT '',
  idempotency_key text NOT NULL CHECK (char_length(idempotency_key)<=50),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(reminder_id, scheduled_for),
  UNIQUE(idempotency_key)
);
CREATE INDEX IF NOT EXISTS reminder_deliveries_due_idx ON reminder_deliveries(next_attempt_at) WHERE status IN ('pending','retry','sending');

CREATE TABLE IF NOT EXISTS workday_overrides (
  work_date date PRIMARY KEY,
  is_workday boolean NOT NULL,
  note text NOT NULL DEFAULT '',
  updated_by uuid REFERENCES users(id),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS reminder_bot_jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_id text UNIQUE NOT NULL,
  open_id text NOT NULL,
  chat_id text NOT NULL,
  message_id text NOT NULL DEFAULT '',
  content text NOT NULL,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','done','retry','failed')),
  attempts integer NOT NULL DEFAULT 0,
  last_error text NOT NULL DEFAULT '',
  available_at timestamptz NOT NULL DEFAULT now(),
  locked_until timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS reminder_bot_jobs_due_idx ON reminder_bot_jobs(available_at) WHERE status IN ('pending','retry','processing');
