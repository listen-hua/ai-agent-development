CREATE TABLE IF NOT EXISTS local_permission_policies (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  allow_keys text[] NOT NULL DEFAULT '{}',
  deny_keys text[] NOT NULL DEFAULT '{}',
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT local_permission_allow_keys_valid CHECK (
    allow_keys <@ ARRAY[
      'agent_use','knowledge_manage','agent_manage','image_manage',
      'notification_manage','calendar_manage','audit_view','user_manage'
    ]::text[]
  ),
  CONSTRAINT local_permission_deny_keys_valid CHECK (
    deny_keys <@ ARRAY[
      'agent_use','knowledge_manage','agent_manage','image_manage',
      'notification_manage','calendar_manage','audit_view','user_manage'
    ]::text[]
  ),
  CONSTRAINT local_permission_sets_disjoint CHECK (NOT allow_keys && deny_keys)
);

CREATE TABLE IF NOT EXISTS iam_permission_snapshots (
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  iam_user_id bigint,
  permission_keys text[] NOT NULL DEFAULT '{}',
  policy_version text NOT NULL DEFAULT '',
  source_updated_at timestamptz,
  synced_at timestamptz NOT NULL DEFAULT now(),
  last_error text NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS iam_permission_snapshots_iam_user_id_idx
  ON iam_permission_snapshots(iam_user_id)
  WHERE iam_user_id IS NOT NULL;

-- Legacy roles are retained read-only for one release. Their administrative
-- meaning is migrated once into explicit local allows. employee intentionally
-- does not become a local agent_use allow, so IAM can still revoke access.
WITH role_permissions(role, permission_key) AS (
  VALUES
    ('knowledge_admin', 'knowledge_manage'),
    ('knowledge_admin', 'agent_manage'),
    ('knowledge_admin', 'audit_view'),
    ('notification_admin', 'notification_manage'),
    ('notification_admin', 'calendar_manage'),
    ('notification_admin', 'audit_view'),
    ('image_admin', 'image_manage'),
    ('auditor', 'audit_view'),
    ('super_admin', 'agent_use'),
    ('super_admin', 'knowledge_manage'),
    ('super_admin', 'agent_manage'),
    ('super_admin', 'image_manage'),
    ('super_admin', 'notification_manage'),
    ('super_admin', 'calendar_manage'),
    ('super_admin', 'audit_view'),
    ('super_admin', 'user_manage')
),
migrated AS (
  SELECT ra.user_id, array_agg(DISTINCT rp.permission_key ORDER BY rp.permission_key)::text[] AS allow_keys
  FROM role_assignments ra
  JOIN role_permissions rp ON rp.role = ra.role
  GROUP BY ra.user_id
)
INSERT INTO local_permission_policies(user_id, allow_keys, deny_keys, reason)
SELECT user_id, allow_keys, '{}', 'Migrated from legacy roles'
FROM migrated
ON CONFLICT (user_id) DO NOTHING;

-- Convert explicit legacy ACL role names to the matching permission key while
-- preserving all user, department, job and rule based restrictions.
WITH mapped AS (
  SELECT d.id,
    COALESCE(
      (
        SELECT jsonb_agg(DISTINCT permission_key)
        FROM (
          SELECT CASE value
            WHEN 'employee' THEN 'agent_use'
            WHEN 'knowledge_admin' THEN 'knowledge_manage'
            WHEN 'notification_admin' THEN 'notification_manage'
            WHEN 'image_admin' THEN 'image_manage'
            WHEN 'auditor' THEN 'audit_view'
            WHEN 'super_admin' THEN 'user_manage'
          END AS permission_key
          FROM jsonb_array_elements_text(COALESCE(d.acl->'role_names', '[]'::jsonb))
          UNION ALL
          SELECT value FROM jsonb_array_elements_text(COALESCE(d.acl->'permission_keys', '[]'::jsonb))
        ) values_to_map
        WHERE permission_key IS NOT NULL
      ),
      '[]'::jsonb
    ) AS permission_keys
  FROM documents d
  WHERE d.acl ? 'role_names'
)
UPDATE documents d
SET acl = (d.acl - 'role_names') || jsonb_build_object('permission_keys', mapped.permission_keys)
FROM mapped
WHERE d.id = mapped.id;

WITH mapped AS (
  SELECT s.id,
    COALESCE(
      (
        SELECT jsonb_agg(DISTINCT permission_key)
        FROM (
          SELECT CASE value
            WHEN 'employee' THEN 'agent_use'
            WHEN 'knowledge_admin' THEN 'knowledge_manage'
            WHEN 'notification_admin' THEN 'notification_manage'
            WHEN 'image_admin' THEN 'image_manage'
            WHEN 'auditor' THEN 'audit_view'
            WHEN 'super_admin' THEN 'user_manage'
          END AS permission_key
          FROM jsonb_array_elements_text(COALESCE(s.default_acl->'role_names', '[]'::jsonb))
          UNION ALL
          SELECT value FROM jsonb_array_elements_text(COALESCE(s.default_acl->'permission_keys', '[]'::jsonb))
        ) values_to_map
        WHERE permission_key IS NOT NULL
      ),
      '[]'::jsonb
    ) AS permission_keys
  FROM knowledge_sources s
  WHERE s.default_acl ? 'role_names'
)
UPDATE knowledge_sources s
SET default_acl = (s.default_acl - 'role_names') || jsonb_build_object('permission_keys', mapped.permission_keys)
FROM mapped
WHERE s.id = mapped.id;

WITH mapped AS (
  SELECT p.id,
    COALESCE(
      (
        SELECT jsonb_agg(DISTINCT permission_key)
        FROM (
          SELECT CASE value
            WHEN 'employee' THEN 'agent_use'
            WHEN 'knowledge_admin' THEN 'knowledge_manage'
            WHEN 'notification_admin' THEN 'notification_manage'
            WHEN 'image_admin' THEN 'image_manage'
            WHEN 'auditor' THEN 'audit_view'
            WHEN 'super_admin' THEN 'user_manage'
          END AS permission_key
          FROM jsonb_array_elements_text(COALESCE(p.acl->'role_names', '[]'::jsonb))
          UNION ALL
          SELECT value FROM jsonb_array_elements_text(COALESCE(p.acl->'permission_keys', '[]'::jsonb))
        ) values_to_map
        WHERE permission_key IS NOT NULL
      ),
      '[]'::jsonb
    ) AS permission_keys
  FROM image_projects p
  WHERE p.acl ? 'role_names'
)
UPDATE image_projects p
SET acl = (p.acl - 'role_names') || jsonb_build_object('permission_keys', mapped.permission_keys)
FROM mapped
WHERE p.id = mapped.id;
