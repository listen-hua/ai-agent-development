ALTER TABLE image_jobs DROP CONSTRAINT IF EXISTS image_jobs_aspect_ratio_check;
ALTER TABLE image_jobs ADD CONSTRAINT image_jobs_aspect_ratio_check
  CHECK (aspect_ratio IN ('original','1:1','16:9','9:16','4:3','3:4'));
