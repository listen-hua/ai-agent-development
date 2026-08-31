ALTER TABLE image_prompt_actions
  ADD COLUMN IF NOT EXISTS project_ids uuid[] NOT NULL DEFAULT '{}';

UPDATE image_prompt_actions
SET project_ids = ARRAY[project_id]
WHERE project_id IS NOT NULL
  AND cardinality(project_ids) = 0;

CREATE INDEX IF NOT EXISTS image_prompt_actions_project_ids_idx
  ON image_prompt_actions USING gin(project_ids);
