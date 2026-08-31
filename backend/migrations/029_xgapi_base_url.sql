UPDATE image_relays
SET base_url = 'https://api.xgapiproxy.win/v1',
    allowed_output_hosts = (
      SELECT array_agg(DISTINCT host ORDER BY host)
      FROM unnest(
        array_append(
          array_remove(COALESCE(image_relays.allowed_output_hosts, ARRAY[]::text[]), 'api.xgapi.top'),
          'api.xgapiproxy.win'
        )
      ) AS host
    ),
    updated_at = NOW()
WHERE relay_key = 'xgapi'
  AND (
    base_url IS DISTINCT FROM 'https://api.xgapiproxy.win/v1'
    OR 'api.xgapi.top' = ANY(COALESCE(allowed_output_hosts, ARRAY[]::text[]))
    OR NOT ('api.xgapiproxy.win' = ANY(COALESCE(allowed_output_hosts, ARRAY[]::text[])))
  );
