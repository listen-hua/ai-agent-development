CREATE TABLE IF NOT EXISTS meeting_rooms (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  room_id text UNIQUE NOT NULL,
  name text NOT NULL,
  capacity integer NOT NULL DEFAULT 0,
  room_level_id text NOT NULL DEFAULT '',
  path jsonb NOT NULL DEFAULT '[]',
  enabled boolean NOT NULL DEFAULT false,
  schedule_enabled boolean NOT NULL DEFAULT false,
  disabled_from timestamptz,
  disabled_until timestamptz,
  disable_reason text NOT NULL DEFAULT '',
  approval_switch integer NOT NULL DEFAULT 0,
  approval_condition integer NOT NULL DEFAULT 0,
  approval_duration_hours double precision NOT NULL DEFAULT 0,
  reservation_start_seconds integer NOT NULL DEFAULT 32400,
  reservation_end_seconds integer NOT NULL DEFAULT 64800,
  max_duration_hours integer NOT NULL DEFAULT 2,
  last_error text NOT NULL DEFAULT '',
  synced_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS meeting_rooms_recommend_idx ON meeting_rooms(enabled, schedule_enabled, capacity);

CREATE TABLE IF NOT EXISTS meeting_settings (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  calendar_id text NOT NULL DEFAULT '',
  timezone text NOT NULL DEFAULT 'Asia/Shanghai',
  workday_start text NOT NULL DEFAULT '09:00',
  workday_end text NOT NULL DEFAULT '18:00',
  slot_minutes integer NOT NULL DEFAULT 30,
  sync_interval_minutes integer NOT NULL DEFAULT 15,
  last_synced_at timestamptz,
  last_sync_error text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO meeting_settings(singleton) VALUES(true) ON CONFLICT(singleton) DO NOTHING;

CREATE TABLE IF NOT EXISTS meeting_bookings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  calendar_id text NOT NULL,
  event_id text UNIQUE NOT NULL,
  room_id text NOT NULL,
  room_name text NOT NULL,
  title text NOT NULL,
  start_at timestamptz NOT NULL,
  end_at timestamptz NOT NULL,
  attendees jsonb NOT NULL DEFAULT '[]',
  status text NOT NULL CHECK (status IN ('active','cancelled','replaced','needs_admin')),
  replaces_booking_id uuid REFERENCES meeting_bookings(id),
  replaced_by_booking_id uuid REFERENCES meeting_bookings(id),
  last_error text NOT NULL DEFAULT '',
  source_channel text NOT NULL DEFAULT 'h5' CHECK (source_channel IN ('h5','feishu_bot')),
  source_conversation_id uuid REFERENCES conversations(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS meeting_bookings_user_idx ON meeting_bookings(user_id, start_at DESC);
CREATE INDEX IF NOT EXISTS meeting_bookings_active_idx ON meeting_bookings(end_at) WHERE status='active';

CREATE TABLE IF NOT EXISTS meeting_booking_actions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id),
  intent text NOT NULL CHECK (intent IN ('create','cancel','reschedule')),
  title text NOT NULL DEFAULT '',
  attendees jsonb NOT NULL DEFAULT '[]',
  capacity integer NOT NULL DEFAULT 1,
  requested_room_name text NOT NULL DEFAULT '',
  options jsonb NOT NULL DEFAULT '[]',
  selected_option_id uuid,
  booking_id uuid REFERENCES meeting_bookings(id),
  result_booking_id uuid REFERENCES meeting_bookings(id),
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','confirmed','cancelled','expired')),
  source_channel text NOT NULL DEFAULT 'h5' CHECK (source_channel IN ('h5','feishu_bot')),
  source_conversation_id uuid REFERENCES conversations(id) ON DELETE SET NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  confirmed_at timestamptz
);
CREATE INDEX IF NOT EXISTS meeting_booking_actions_user_idx ON meeting_booking_actions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS meeting_booking_actions_expiry_idx ON meeting_booking_actions(expires_at) WHERE status='pending';

ALTER TABLE messages ADD COLUMN IF NOT EXISTS meeting_booking_action_id uuid REFERENCES meeting_booking_actions(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS meeting_booking_deliveries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id uuid NOT NULL REFERENCES meeting_bookings(id) ON DELETE CASCADE,
  scheduled_for timestamptz NOT NULL,
  status text NOT NULL CHECK (status IN ('pending','sending','retry','sent','cancelled','failed')),
  attempts integer NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL,
  locked_until timestamptz,
  message_id text NOT NULL DEFAULT '',
  error text NOT NULL DEFAULT '',
  idempotency_key text UNIQUE NOT NULL CHECK (char_length(idempotency_key)<=50),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(booking_id, scheduled_for)
);
CREATE INDEX IF NOT EXISTS meeting_booking_deliveries_due_idx ON meeting_booking_deliveries(next_attempt_at) WHERE status IN ('pending','retry','sending');
