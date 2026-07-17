ALTER TABLE notification_drafts
  ADD COLUMN IF NOT EXISTS content_format text NOT NULL DEFAULT 'markdown',
  ADD COLUMN IF NOT EXISTS images jsonb NOT NULL DEFAULT '[]',
  ADD COLUMN IF NOT EXISTS recipient_type text NOT NULL DEFAULT 'legacy',
  ADD COLUMN IF NOT EXISTS recipients jsonb NOT NULL DEFAULT '[]',
  ADD COLUMN IF NOT EXISTS last_error text NOT NULL DEFAULT '';

ALTER TABLE notification_drafts DROP CONSTRAINT IF EXISTS notification_drafts_recipient_type_check;
ALTER TABLE notification_drafts ADD CONSTRAINT notification_drafts_recipient_type_check
  CHECK (recipient_type IN ('legacy','user','chat'));

ALTER TABLE delivery_attempts
  ADD COLUMN IF NOT EXISTS receiver_type text NOT NULL DEFAULT 'open_id',
  ADD COLUMN IF NOT EXISTS receiver_name text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS notification_drafts_due_idx
  ON notification_drafts(scheduled_at)
  WHERE status='scheduled';
