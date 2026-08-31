ALTER TABLE image_canvas_nodes
  ADD COLUMN IF NOT EXISTS generation_relay_name text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS generation_model_name text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS generation_model_key text NOT NULL DEFAULT '';

UPDATE image_canvas_nodes AS node
SET generation_relay_name = relay.name,
    generation_model_name = model.display_name,
    generation_model_key = model.model_id
FROM image_jobs AS job
JOIN image_relays AS relay ON relay.id = job.relay_id
JOIN image_models AS model ON model.id = job.model_id
WHERE node.job_id = job.id
  AND node.generation_relay_name = '';

UPDATE image_canvas_nodes AS result
SET generation_relay_name = source.generation_relay_name,
    generation_model_name = source.generation_model_name,
    generation_model_key = source.generation_model_key
FROM image_canvas_nodes AS source
WHERE result.source_node_id = source.id
  AND result.background_removal_job_id IS NOT NULL
  AND result.generation_relay_name = '';

UPDATE image_canvas_nodes AS result
SET generation_relay_name = relay.name,
    generation_model_name = model.display_name,
    generation_model_key = model.model_id
FROM image_assets AS result_asset
JOIN image_job_outputs AS source_output ON source_output.asset_id = result_asset.source_asset_id
JOIN image_jobs AS source_job ON source_job.id = source_output.job_id
JOIN image_relays AS relay ON relay.id = source_job.relay_id
JOIN image_models AS model ON model.id = source_job.model_id
WHERE result.asset_id = result_asset.id
  AND result.background_removal_job_id IS NOT NULL
  AND result.generation_relay_name = '';
