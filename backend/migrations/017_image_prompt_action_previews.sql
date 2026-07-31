ALTER TABLE image_prompt_actions
  ADD COLUMN IF NOT EXISTS preview_object_key text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS preview_mime_type text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS preview_size_bytes bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS preview_width integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS preview_height integer NOT NULL DEFAULT 0;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'image_prompt_actions_preview_valid'
  ) THEN
    ALTER TABLE image_prompt_actions
      ADD CONSTRAINT image_prompt_actions_preview_valid CHECK (
        (
          preview_object_key = '' AND preview_mime_type = '' AND preview_size_bytes = 0
          AND preview_width = 0 AND preview_height = 0
        ) OR (
          preview_object_key <> ''
          AND preview_mime_type IN ('image/jpeg','image/png','image/webp')
          AND preview_size_bytes BETWEEN 1 AND 5242880
          AND preview_width > 0 AND preview_height > 0
        )
      );
  END IF;
END $$;
