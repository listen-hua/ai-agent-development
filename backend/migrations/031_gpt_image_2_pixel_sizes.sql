-- GPT Image 2 uses concrete WIDTHxHEIGHT values through the Images API.
-- Keep relay model synchronization and existing records on the same protocol.
UPDATE image_models AS model
SET protocol = 'gpt_image_2',
    request_model_id = CASE WHEN model.request_model_id = '' THEN model.model_id ELSE model.request_model_id END,
    supports_reference = true,
    supports_reverse = false,
    supported_sizes = ARRAY['1K','2K','4K'],
    updated_at = NOW()
FROM image_relays AS relay
WHERE relay.id = model.relay_id
  AND relay.relay_key IN ('comfly','xgapi')
  AND lower(regexp_replace(model.model_id, '^openai/', '')) = 'gpt-image-2';
