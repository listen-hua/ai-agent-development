UPDATE image_relays
SET allowed_output_hosts = array_append(allowed_output_hosts, 'image.xgapiproxy.win'),
    updated_at = now()
WHERE relay_key = 'xgapi'
  AND NOT ('image.xgapiproxy.win' = ANY(allowed_output_hosts));
