ALTER TABLE users ADD COLUMN IF NOT EXISTS job_title text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS job_level_id text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS job_family_id text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS employee_type integer NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS organization_synced_at timestamptz;

CREATE INDEX IF NOT EXISTS users_department_ids_idx ON users USING gin(department_ids);
CREATE INDEX IF NOT EXISTS users_job_title_idx ON users(job_title);
CREATE INDEX IF NOT EXISTS documents_acl_idx ON documents USING gin(acl jsonb_path_ops);
