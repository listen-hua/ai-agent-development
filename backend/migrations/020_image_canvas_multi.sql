ALTER TABLE image_canvases ADD COLUMN IF NOT EXISTS name text;
ALTER TABLE image_canvases ADD COLUMN IF NOT EXISTS deleted_at timestamptz;

WITH named AS (
  SELECT c.id,
         left(btrim(p.name), 70) AS project_name,
         row_number() OVER (PARTITION BY c.user_id, lower(left(btrim(p.name), 70)) ORDER BY c.created_at, c.id) AS duplicate_no
  FROM image_canvases c
  JOIN image_projects p ON p.id = c.project_id
  WHERE c.name IS NULL OR btrim(c.name) = ''
)
UPDATE image_canvases c
SET name = CASE WHEN named.duplicate_no = 1 THEN named.project_name
                ELSE named.project_name || '（' || named.duplicate_no || '）' END
FROM named WHERE named.id = c.id;

UPDATE image_canvases
SET name = '我的画布-' || left(id::text, 8)
WHERE name IS NULL OR btrim(name) = '';

UPDATE image_canvases SET name = left(name, 80) WHERE char_length(name) > 80;

ALTER TABLE image_canvases ALTER COLUMN name SET NOT NULL;
ALTER TABLE image_canvases ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE image_canvases DROP CONSTRAINT IF EXISTS image_canvases_user_id_project_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS image_canvases_legacy_project_unique
  ON image_canvases(user_id, project_id) WHERE project_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS image_canvases_active_name_unique
  ON image_canvases(user_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS image_canvases_user_updated_idx
  ON image_canvases(user_id, updated_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS image_canvases_deleted_idx
  ON image_canvases(deleted_at) WHERE deleted_at IS NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'image_canvases_name_length_check') THEN
    ALTER TABLE image_canvases ADD CONSTRAINT image_canvases_name_length_check
      CHECK (char_length(btrim(name)) BETWEEN 1 AND 80);
  END IF;
END $$;

ALTER TABLE image_jobs DROP CONSTRAINT IF EXISTS image_jobs_canvas_id_fkey;
ALTER TABLE image_jobs ALTER COLUMN canvas_id DROP NOT NULL;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'image_jobs_canvas_id_fkey') THEN
    ALTER TABLE image_jobs ADD CONSTRAINT image_jobs_canvas_id_fkey
      FOREIGN KEY (canvas_id) REFERENCES image_canvases(id) ON DELETE SET NULL;
  END IF;
END $$;
