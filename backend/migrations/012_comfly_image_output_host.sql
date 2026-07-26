UPDATE image_relays
SET allowed_output_hosts = array_append(allowed_output_hosts, 'files.closeai.fans'),
    updated_at = now()
WHERE relay_key = 'comfly'
  AND NOT ('files.closeai.fans' = ANY(allowed_output_hosts));
