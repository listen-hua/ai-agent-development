ALTER TABLE knowledge_sources
  DROP CONSTRAINT IF EXISTS knowledge_sources_type_check;

ALTER TABLE knowledge_sources
  ADD CONSTRAINT knowledge_sources_type_check
  CHECK (type IN ('upload', 'feishu_folder', 'feishu_wiki'));
