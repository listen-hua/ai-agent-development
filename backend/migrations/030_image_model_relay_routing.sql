ALTER TABLE image_models
  ADD COLUMN IF NOT EXISTS request_model_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS remote_endpoint_types text[] NOT NULL DEFAULT ARRAY[]::text[];

UPDATE image_models
SET request_model_id = model_id
WHERE request_model_id = '';

-- Comfly's GPT Image 2 Images endpoint requires a concrete WIDTHxHEIGHT size.
-- Keep it on the dedicated adapter rather than Gemini's Chat image_config.
UPDATE image_models AS model
SET protocol = 'gpt_image_2',
    request_model_id = model.model_id,
    remote_endpoint_types = ARRAY['openai'],
    supports_reference = true,
    supports_reverse = false,
    updated_at = NOW()
FROM image_relays AS relay
WHERE relay.id = model.relay_id
  AND relay.relay_key = 'comfly'
  AND lower(regexp_replace(model.model_id, '^openai/', '')) = 'gpt-image-2';

-- XGAPI's universal edit endpoint uses a base model plus quality=1K/2K/4K.
-- Resolution-suffixed aliases are listed for Chat/Gemini endpoints but do not
-- have an image-generation channel on the new gateway.
UPDATE image_models AS model
SET request_model_id = regexp_replace(model.model_id, '-(1k|2k|4k)$', '', 'i'),
    remote_endpoint_types = ARRAY['gemini','openai'],
    supports_reference = true,
    updated_at = NOW()
FROM image_relays AS relay
WHERE relay.id = model.relay_id
  AND relay.relay_key = 'xgapi'
  AND lower(model.model_id) ~ '^gemini-3\.1-flash-image-preview-(1k|2k|4k)$';

UPDATE image_models AS model
SET remote_endpoint_types = ARRAY['gemini','image-generation','openai'],
    updated_at = NOW()
FROM image_relays AS relay
WHERE relay.id = model.relay_id
  AND relay.relay_key = 'xgapi'
  AND lower(model.model_id) = 'gemini-3.1-flash-image-preview';

UPDATE image_models AS model
SET remote_endpoint_types = ARRAY['image-generation','openai'],
    updated_at = NOW()
FROM image_relays AS relay
WHERE relay.id = model.relay_id
  AND relay.relay_key = 'xgapi'
  AND lower(regexp_replace(model.model_id, '^openai/', '')) = 'gpt-image-2';
