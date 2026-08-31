UPDATE image_relays
SET allowed_output_hosts = array_append(allowed_output_hosts, 'webstatic.apiproxy.vip'),
    updated_at = now()
WHERE relay_key = 'xgapi'
  AND NOT ('webstatic.apiproxy.vip' = ANY(allowed_output_hosts));
