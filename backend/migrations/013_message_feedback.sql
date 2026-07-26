CREATE TABLE IF NOT EXISTS message_feedback (
  message_id uuid PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  positive boolean NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS message_feedback_user_idx
  ON message_feedback(user_id, updated_at DESC);
