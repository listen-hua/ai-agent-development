import type { ACL } from './domain'

export type ImageProtocol = 'chat_completions' | 'images_generations' | 'gpt_image_2'
export type ImageSize = '1K' | '2K' | '4K'
export type ImageRatio = 'original' | '1:1' | '16:9' | '9:16' | '4:3' | '3:4'
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
  request_model_id?: string
  remote_endpoint_types?: string[]
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
  project_ids?: string[]
  enabled: boolean
  sort_order: number
  has_preview: boolean
  preview_mime_type?: string
  preview_size_bytes?: number
  preview_width?: number
  preview_height?: number
  created_at: string
  updated_at: string
}

export interface ImageViewport { x: number; y: number; zoom: number }

export interface ImageCanvasNode {
  id: string
  canvas_id: string
  asset_id?: string
  job_id?: string
  background_removal_job_id?: string
  source_node_id?: string
  output_index: number
  status: 'pending' | 'ready' | 'failed'
  x: number
  y: number
  width: number
  height: number
  z_index: number
  error?: string
  requested_size?: ImageSize
  actual_width?: number
  actual_height?: number
  resolution_warning?: string
  generation_relay_name?: string
  generation_model_name?: string
  generation_model_key?: string
  created_at: string
  updated_at: string
}

export interface ImageCanvas {
  id: string
  user_id: string
  project_id?: string
  name: string
  viewport: ImageViewport
  version: number
  node_count: number
  preview_asset_id?: string
  deleted_at?: string
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
  source: 'upload' | 'generated' | 'background_removed'
  source_asset_id?: string
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
  requested_size?: ImageSize
  actual_width?: number
  actual_height?: number
  resolution_warning?: string
}

export interface ImageJob {
  id: string
  user_id: string
  project_id: string
  canvas_id?: string
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
  background_removal_enabled: boolean
}

export interface BackgroundRemovalItem {
  id: string
  source_node_id: string
  source_asset_id: string
  project_id: string
  placeholder_node_id: string
  result_asset_id?: string
  status: 'pending' | 'running' | 'succeeded' | 'failed'
  attempts: number
  credits_charged: number
  credits_calculated: number
  input_size?: string
  result_size?: string
  error?: string
}

export interface BackgroundRemovalJob {
  id: string
  canvas_id: string
  status: 'pending' | 'running' | 'retry' | 'partial' | 'succeeded' | 'failed' | 'cancelled'
  test_mode: boolean
  attempts: number
  completed_count: number
  failed_count: number
  credits_charged: number
  credits_calculated: number
  error?: string
  items: BackgroundRemovalItem[]
  created_at: string
  updated_at: string
}

export interface PixianBackgroundRemovalConfig {
  enabled: boolean
  test_mode: boolean
  api_id_hint: string
  api_secret_hint: string
  has_api_id: boolean
  has_api_secret: boolean
  timeout_seconds: number
  concurrency: number
  max_pixels: number
  account_state?: string
  account_credits: number
  account_checked_at?: string
  updated_at: string
}

export interface BackgroundRemovalStatistics {
  today_calls: number
  thirty_day_succeeded: number
  thirty_day_failed: number
  thirty_day_images: number
  credits_charged: number
  credits_calculated: number
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
  project_ids: string[]
  enabled: boolean
  sort_order: number
}

export interface ImagePromptActionSaveResult {
  action: ImagePromptAction
  preview_error?: boolean
}

export interface ImagePromptActionPreviewChange {
  file?: File
  remove: boolean
}
