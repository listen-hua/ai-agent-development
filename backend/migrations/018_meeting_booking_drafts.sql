CREATE TABLE IF NOT EXISTS meeting_booking_drafts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  conversation_id uuid REFERENCES conversations(id) ON DELETE CASCADE,
  channel text NOT NULL DEFAULT 'h5' CHECK (channel IN ('h5','feishu_bot')),
  intent text NOT NULL CHECK (intent IN ('create','cancel','reschedule')),
  slots jsonb NOT NULL DEFAULT '{}',
  missing_fields text[] NOT NULL DEFAULT '{}',
  status text NOT NULL DEFAULT 'collecting' CHECK (status IN ('collecting','completed','cancelled','expired')),
  version bigint NOT NULL DEFAULT 1,
  result_action_id uuid REFERENCES meeting_booking_actions(id) ON DELETE SET NULL,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS meeting_booking_drafts_active_idx
  ON meeting_booking_drafts(user_id, conversation_id, updated_at DESC)
  WHERE status='collecting';

ALTER TABLE messages ADD COLUMN IF NOT EXISTS meeting_booking_draft_id uuid REFERENCES meeting_booking_drafts(id) ON DELETE SET NULL;
