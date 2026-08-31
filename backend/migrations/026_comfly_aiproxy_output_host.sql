UPDATE image_relays
SET allowed_output_hosts = array_append(allowed_output_hosts, 'webstatic.aiproxy.vip'),
    updated_at = now()
WHERE relay_key = 'comfly'
  AND NOT ('webstatic.aiproxy.vip' = ANY(allowed_output_hosts));
