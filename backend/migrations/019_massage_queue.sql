CREATE TABLE IF NOT EXISTS massage_cycles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  service_month text NOT NULL,
  title text NOT NULL,
  signup_notice_at timestamptz NOT NULL,
  signup_deadline timestamptz NOT NULL,
  audience jsonb NOT NULL DEFAULT '{"scope":"all"}',
  status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','scheduled','signup_open','in_progress','completed','cancelled')),
  next_queue integer NOT NULL DEFAULT 0,
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(service_month)
);

CREATE TABLE IF NOT EXISTS massage_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  cycle_id uuid NOT NULL REFERENCES massage_cycles(id) ON DELETE CASCADE,
  sequence integer NOT NULL CHECK (sequence IN (1,2)),
  starts_at timestamptz NOT NULL,
  quota integer NOT NULL CHECK (quota > 0),
  concurrent_slots integer NOT NULL CHECK (concurrent_slots BETWEEN 1 AND 20),
  status text NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled','running','paused','completed','closed')),
  completed_count integer NOT NULL DEFAULT 0,
  started_at timestamptz,
  closed_at timestamptz,
  close_reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(cycle_id, sequence)
);

CREATE TABLE IF NOT EXISTS massage_eligible_users (
  cycle_id uuid NOT NULL REFERENCES massage_cycles(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(cycle_id,user_id)
);

CREATE TABLE IF NOT EXISTS massage_enrollments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  cycle_id uuid NOT NULL REFERENCES massage_cycles(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  queue_number integer,
  status text NOT NULL CHECK (status IN ('enrolled','declined','withdrawn','completed','unserved')),
  enrolled_at timestamptz,
  withdrawn_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(cycle_id,user_id)
);

CREATE TABLE IF NOT EXISTS massage_calls (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id uuid NOT NULL REFERENCES massage_sessions(id) ON DELETE CASCADE,
  enrollment_id uuid NOT NULL REFERENCES massage_enrollments(id) ON DELETE CASCADE,
  status text NOT NULL CHECK (status IN ('pending_delivery','awaiting_response','accepted','completed','rejected','timed_out','no_show','delivery_failed','cancelled')),
  called_at timestamptz,
  response_due_at timestamptz,
  responded_at timestamptz,
  completed_at timestamptz,
  message_id text NOT NULL DEFAULT '',
  last_error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(session_id,enrollment_id)
);

CREATE TABLE IF NOT EXISTS massage_deliveries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  cycle_id uuid NOT NULL REFERENCES massage_cycles(id) ON DELETE CASCADE,
  session_id uuid REFERENCES massage_sessions(id) ON DELETE CASCADE,
  call_id uuid REFERENCES massage_calls(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('signup','signup_resend','call')),
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sending','retry','sent','failed','cancelled')),
  attempts integer NOT NULL DEFAULT 0,
  available_at timestamptz NOT NULL,
  locked_until timestamptz,
  message_id text NOT NULL DEFAULT '',
  last_error text NOT NULL DEFAULT '',
  idempotency_key text UNIQUE NOT NULL CHECK (char_length(idempotency_key) <= 50),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS massage_delivery_due_idx ON massage_deliveries(available_at) WHERE status IN ('pending','retry','sending');
CREATE INDEX IF NOT EXISTS massage_call_due_idx ON massage_calls(response_due_at) WHERE status='awaiting_response';
CREATE INDEX IF NOT EXISTS massage_enrollment_queue_idx ON massage_enrollments(cycle_id,queue_number) WHERE status='enrolled';
CREATE UNIQUE INDEX IF NOT EXISTS massage_enrollment_number_unique_idx ON massage_enrollments(cycle_id,queue_number) WHERE queue_number IS NOT NULL;

CREATE TABLE IF NOT EXISTS massage_monthly_statistics (
  service_month text PRIMARY KEY,
  eligible_count integer NOT NULL DEFAULT 0,
  enrolled_count integer NOT NULL DEFAULT 0,
  completed_count integer NOT NULL DEFAULT 0,
  rejected_count integer NOT NULL DEFAULT 0,
  timed_out_count integer NOT NULL DEFAULT 0,
  no_show_count integer NOT NULL DEFAULT 0,
  delivery_failed_count integer NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE messages ADD COLUMN IF NOT EXISTS massage_action jsonb;
