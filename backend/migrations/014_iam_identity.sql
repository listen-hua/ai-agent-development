ALTER TABLE users ADD COLUMN IF NOT EXISTS iam_user_id bigint;
ALTER TABLE users ADD COLUMN IF NOT EXISTS feishu_user_id text;

CREATE UNIQUE INDEX IF NOT EXISTS users_iam_user_id_unique
  ON users(iam_user_id)
  WHERE iam_user_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS users_feishu_user_id_unique
  ON users(feishu_user_id)
  WHERE feishu_user_id IS NOT NULL AND feishu_user_id <> '';
