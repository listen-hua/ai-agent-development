ALTER TABLE image_models DROP CONSTRAINT IF EXISTS image_models_protocol_check;
ALTER TABLE image_models ADD CONSTRAINT image_models_protocol_check
  CHECK (protocol IN ('chat_completions','images_generations','gpt_image_2'));

ALTER TABLE image_job_outputs
  ADD COLUMN IF NOT EXISTS requested_size text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS actual_width integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS actual_height integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS resolution_warning text NOT NULL DEFAULT '';

ALTER TABLE image_canvas_nodes
  ADD COLUMN IF NOT EXISTS requested_size text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS actual_width integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS actual_height integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS resolution_warning text NOT NULL DEFAULT '';

UPDATE image_models
SET protocol='gpt_image_2',
    supports_reference=true,
    supports_reverse=false,
    supported_sizes=ARRAY['1K','2K','4K'],
    updated_at=now()
WHERE lower(regexp_replace(model_id, '^openai/', '')) = 'gpt-image-2';

UPDATE image_relays
SET timeout_seconds=360, updated_at=now()
WHERE relay_key IN ('xgapi','comfly') AND timeout_seconds=120;
