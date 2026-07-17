CREATE TABLE IF NOT EXISTS agent_profiles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_key text UNIQUE NOT NULL,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  kind text NOT NULL CHECK (kind IN ('chat','image')),
  provider text NOT NULL,
  model text NOT NULL,
  enabled boolean NOT NULL DEFAULT true,
  credential_source text NOT NULL DEFAULT 'environment' CHECK (credential_source IN ('environment','database')),
  encrypted_api_key text NOT NULL DEFAULT '',
  api_key_hint text NOT NULL DEFAULT '',
  settings jsonb NOT NULL DEFAULT '{}',
  created_by uuid REFERENCES users(id),
  updated_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO agent_profiles(id,agent_key,name,description,kind,provider,model,enabled,credential_source)
VALUES
  ('00000000-0000-4000-8000-000000000101','administrative_assistant','行政助手','基于公司制度知识库回答行政问题','chat','aliyun','qwen-plus',true,'environment'),
  ('00000000-0000-4000-8000-000000000102','image_generator','AI 生图','在画布中使用 Gemini 生成和编辑图片','image','gemini','gemini-3.1-flash-image',true,'environment')
ON CONFLICT(agent_key) DO NOTHING;

-- Repair seed labels damaged by Windows shell pipelines that replaced UTF-8
-- characters with literal question marks. Never overwrite valid custom labels.
UPDATE agent_profiles
SET name = CASE WHEN name = repeat(chr(63), 4) THEN '行政助手' ELSE name END,
    description = CASE WHEN description = repeat(chr(63), 15) THEN '基于公司制度知识库回答行政问题' ELSE description END,
    updated_at = now()
WHERE agent_key = 'administrative_assistant'
  AND (name = repeat(chr(63), 4) OR description = repeat(chr(63), 15));

UPDATE agent_profiles
SET name = CASE WHEN name = 'AI ' || repeat(chr(63), 2) THEN 'AI 生图' ELSE name END,
    description = CASE WHEN description = repeat(chr(63), 6) || ' Gemini ' || repeat(chr(63), 7) THEN '在画布中使用 Gemini 生成和编辑图片' ELSE description END,
    updated_at = now()
WHERE agent_key = 'image_generator'
  AND (name = 'AI ' || repeat(chr(63), 2) OR description = repeat(chr(63), 6) || ' Gemini ' || repeat(chr(63), 7));
