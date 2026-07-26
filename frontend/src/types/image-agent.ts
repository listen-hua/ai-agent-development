import type { ACL } from './domain'

export type ImageProtocol = 'chat_completions' | 'images_generations'
export type ImageSize = '1K' | '2K' | '4K'
export type ImageRatio = '1:1' | '16:9' | '9:16' | '4:3' | '3:4'
export type ImageCount = 1 | 2 | 4 | 8

export interface ImageRelay {
  id: string
  relay_key: string
  name: string
  base_url: string
  enabled: boolean
  timeout_seconds: number
  allowed_output_hosts: string[]
  has_api_key: boolean
  api_key_hint: string
  created_at: string
  updated_at: string
}

export interface ImageModel {
  id: string
  relay_id: string
  model_id: string
  display_name: string
  protocol: ImageProtocol
  enabled: boolean
  supports_reference: boolean
  supports_reverse: boolean
  supported_sizes: ImageSize[]
  max_count: ImageCount
  created_at: string
  updated_at: string
}

export interface ImageProject {
  id: string
  project_key: string
  name: string
  description: string
  acl: ACL
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface ImagePromptAction {
  id: string
  action_key: string
  name: string
  prompt_template: string
  project_id?: string
  enabled: boolean
  sort_order: number
  created_at: string
  updated_at: string
}

export interface ImageViewport { x: number; y: number; zoom: number }

export interface ImageCanvasNode {
  id: string
  canvas_id: string
  asset_id?: string
  job_id?: string
  output_index: number
  status: 'pending' | 'ready' | 'failed'
  x: number
  y: number
  width: number
  height: number
  z_index: number
  error?: string
  created_at: string
  updated_at: string
}

export interface ImageCanvas {
  id: string
  user_id: string
  project_id: string
  viewport: ImageViewport
  version: number
  nodes: ImageCanvasNode[]
  created_at: string
  updated_at: string
}

export interface ImageAsset {
  id: string
  owner_id: string
  project_id: string
  mime_type: string
  file_name: string
  width: number
  height: number
  size_bytes: number
  source: 'upload' | 'generated'
  created_at: string
}

export interface ImageJobOutput {
  id: string
  job_id: string
  output_index: number
  asset_id?: string
  status: 'pending' | 'running' | 'succeeded' | 'failed'
  error?: string
  attempts: number
}

export interface ImageJob {
  id: string
  user_id: string
  project_id: string
  canvas_id: string
  relay_id: string
  model_id: string
  kind: 'generate' | 'reverse_prompt'
  prompt: string
  aspect_ratio: ImageRatio
  image_size: ImageSize
  count: ImageCount
  reference_asset_ids: string[]
  status: 'pending' | 'running' | 'retry' | 'partial' | 'succeeded' | 'failed' | 'cancelled'
  attempts: number
  completed_count: number
  reversed_prompt?: string
  error?: string
  outputs: ImageJobOutput[]
  created_at: string
  updated_at: string
}

export interface ImageAgentOptions {
  relays: ImageRelay[]
  models: ImageModel[]
  projects: ImageProject[]
  prompt_actions: ImagePromptAction[]
}

export interface ImageFormState {
  prompt: string
  relayId: string
  modelId: string
  projectId: string
  reversePrompt: boolean
  aspectRatio: ImageRatio
  imageSize: ImageSize
  count: ImageCount
}

export interface ImageRelayInput {
  relay_key: string
  name: string
  base_url: string
  api_key?: string
  enabled: boolean
  timeout_seconds: number
  allowed_output_hosts: string[]
}

export interface ImageModelInput {
  display_name: string
  protocol: ImageProtocol
  enabled: boolean
  supports_reference: boolean
  supports_reverse: boolean
  supported_sizes: ImageSize[]
  max_count: ImageCount
}

export interface ImageProjectInput {
  project_key: string
  name: string
  description: string
  acl: ACL
  enabled: boolean
}

export interface ImagePromptActionInput {
  action_key: string
  name?: string
  prompt_template: string
  project_id?: string
  enabled: boolean
  sort_order: number
}
