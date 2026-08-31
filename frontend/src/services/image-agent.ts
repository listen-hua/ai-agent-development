import { api, resolveApiURL } from './api'
import type {
  ImageAgentOptions, ImageAsset, ImageCanvas, ImageCanvasNode, ImageJob, ImageProjectInput,
  ImagePromptAction, ImagePromptActionInput, ImageRelay, ImageRelayInput, ImageModel, ImageModelInput,
  ImageProject, ImageRatio, ImageSize, ImageCount, ImageViewport,
  BackgroundRemovalJob, PixianBackgroundRemovalConfig, BackgroundRemovalStatistics,
} from '@/types/image-agent'

export const imageAssetURL = (id: string) => resolveApiURL(`/api/v1/image-agent/assets/${id}/content`)
export const imagePromptActionPreviewURL = (id: string, version?: string) => {
  const query = version ? `?v=${encodeURIComponent(version)}` : ''
  return resolveApiURL(`/api/v1/image-agent/prompt-actions/${encodeURIComponent(id)}/preview${query}`)
}

export const imageAgentService = {
  options: (projectId?: string) => api.get<ImageAgentOptions>(`/api/v1/image-agent/options${projectId ? `?project_id=${encodeURIComponent(projectId)}` : ''}`).then(normalizeImageAgentOptions),
  promptActions: (projectId: string) => api.get<ImagePromptAction[]>(`/api/v1/image-agent/prompt-actions?project_id=${encodeURIComponent(projectId)}`),
  canvases: () => api.get<ImageCanvas[]>('/api/v1/image-agent/canvases').then((items) => (items || []).map(normalizeImageCanvas)),
  deletedCanvases: () => api.get<ImageCanvas[]>('/api/v1/image-agent/canvases/trash').then((items) => (items || []).map(normalizeImageCanvas)),
  createCanvas: (name: string) => api.post<ImageCanvas>('/api/v1/image-agent/canvases', { name }).then(normalizeImageCanvas),
  canvasById: (canvasId: string) => api.get<ImageCanvas>(`/api/v1/image-agent/canvases/${encodeURIComponent(canvasId)}`).then(normalizeImageCanvas),
  saveCanvasById: (canvasId: string, input: { viewport: ImageViewport; nodes: ImageCanvasNode[]; version: number }) =>
    api.patch<ImageCanvas>(`/api/v1/image-agent/canvases/${encodeURIComponent(canvasId)}`, input).then(normalizeImageCanvas),
  removeCanvas: (canvasId: string) => api.delete<void>(`/api/v1/image-agent/canvases/${encodeURIComponent(canvasId)}`),
  restoreCanvas: (canvasId: string) => api.post<ImageCanvas>(`/api/v1/image-agent/canvases/${encodeURIComponent(canvasId)}/restore`).then(normalizeImageCanvas),
  deleteCanvasNodeById: (canvasId: string, nodeId: string, version: number) =>
    api.delete<ImageCanvas>(`/api/v1/image-agent/canvases/${encodeURIComponent(canvasId)}/nodes/${encodeURIComponent(nodeId)}?version=${version}`).then(normalizeImageCanvas),
  deleteCanvasNodesById: (canvasId: string, nodeIds: string[], version: number) =>
    api.post<ImageCanvas>(`/api/v1/image-agent/canvases/${encodeURIComponent(canvasId)}/nodes/batch-delete`, {
      node_ids: nodeIds, version,
    }).then(normalizeImageCanvas),
  createBackgroundRemovalJob: (canvasId: string, input: { node_ids: string[]; version: number; idempotency_key: string }) =>
    api.post<BackgroundRemovalJob>(`/api/v1/image-agent/canvases/${encodeURIComponent(canvasId)}/background-removal-jobs`, input),
  backgroundRemovalJob: (id: string) => api.get<BackgroundRemovalJob>(`/api/v1/image-agent/background-removal-jobs/${encodeURIComponent(id)}`),
  importCanvasAssetById(canvasId: string, projectId: string, file: File, input: { x: number; y: number; version: number; origin: 'paste' | 'drop' }) {
    const body = new FormData()
    body.set('file', file)
    body.set('project_id', projectId)
    body.set('x', String(input.x)); body.set('y', String(input.y)); body.set('version', String(input.version)); body.set('origin', input.origin)
    return api.upload<ImageCanvas>(`/api/v1/image-agent/canvases/${encodeURIComponent(canvasId)}/imports`, body).then(normalizeImageCanvas)
  },
  canvas: (projectId: string) => api.get<ImageCanvas>(`/api/v1/image-agent/projects/${projectId}/canvas`).then(normalizeImageCanvas),
  saveCanvas: (projectId: string, input: { viewport: ImageViewport; nodes: ImageCanvasNode[]; version: number }) =>
    api.patch<ImageCanvas>(`/api/v1/image-agent/projects/${projectId}/canvas`, input).then(normalizeImageCanvas),
  deleteCanvasNode: (projectId: string, nodeId: string, version: number) =>
    api.delete<ImageCanvas>(
      `/api/v1/image-agent/projects/${encodeURIComponent(projectId)}/canvas/nodes/${encodeURIComponent(nodeId)}?version=${version}`,
    ).then(normalizeImageCanvas),
  importCanvasAsset(projectId: string, file: File, input: { x: number; y: number; version: number; origin: 'paste' | 'drop' }) {
    const body = new FormData()
    body.set('file', file)
    body.set('x', String(input.x))
    body.set('y', String(input.y))
    body.set('version', String(input.version))
    body.set('origin', input.origin)
    return api.upload<ImageCanvas>(`/api/v1/image-agent/projects/${projectId}/canvas/imports`, body).then(normalizeImageCanvas)
  },
  uploadAsset(file: File, projectId: string) {
    const body = new FormData()
    body.set('file', file)
    body.set('project_id', projectId)
    return api.upload<ImageAsset>('/api/v1/image-agent/assets', body)
  },
  createJob: (input: {
    canvas_id: string
    project_id: string
    relay_id: string
    model_id: string
    kind: 'generate' | 'reverse_prompt'
    prompt: string
    aspect_ratio: ImageRatio
    image_size: ImageSize
    count: ImageCount
    reference_asset_ids: string[]
    idempotency_key: string
    placement_x: number
    placement_y: number
    anchor_node_id?: string
  }) => api.post<ImageJob>('/api/v1/image-agent/jobs', input),
  job: (id: string) => api.get<ImageJob>(`/api/v1/image-agent/jobs/${id}`),
  jobs: (projectId: string) => api.get<ImageJob[]>(`/api/v1/image-agent/jobs?project_id=${encodeURIComponent(projectId)}`),
}

export function normalizeImageAgentOptions(value?: Partial<ImageAgentOptions> | null): ImageAgentOptions {
  return {
    relays: Array.isArray(value?.relays) ? value.relays : [],
    models: Array.isArray(value?.models) ? value.models.map((model) => ({
      ...model,
      request_model_id: model.request_model_id || model.model_id,
      remote_endpoint_types: Array.isArray(model.remote_endpoint_types) ? model.remote_endpoint_types : [],
      supported_sizes: Array.isArray(model.supported_sizes) ? model.supported_sizes : [],
    })) : [],
    projects: Array.isArray(value?.projects) ? value.projects : [],
    prompt_actions: Array.isArray(value?.prompt_actions) ? value.prompt_actions : [],
    background_removal_enabled: Boolean(value?.background_removal_enabled),
  }
}

export function normalizeImageCanvas(value: ImageCanvas): ImageCanvas {
  return {
    ...value,
    viewport: value?.viewport || { x: 0, y: 0, zoom: 1 },
    nodes: Array.isArray(value?.nodes) ? value.nodes : [],
  }
}

export const imageAgentAdminService = {
  relays: () => api.get<ImageRelay[]>('/api/v1/admin/image-agent/relays'),
  createRelay: (input: ImageRelayInput) => api.post<ImageRelay>('/api/v1/admin/image-agent/relays', input),
  updateRelay: (id: string, input: ImageRelayInput) => api.put<ImageRelay>(`/api/v1/admin/image-agent/relays/${id}`, input),
  testRelay: (id: string) => api.post<{ ok: boolean }>(`/api/v1/admin/image-agent/relays/${id}/test`),
  syncModels: (id: string) => api.post<ImageModel[]>(`/api/v1/admin/image-agent/relays/${id}/models/sync`),
  models: (relayId?: string) => api.get<ImageModel[]>(`/api/v1/admin/image-agent/models${relayId ? `?relay_id=${encodeURIComponent(relayId)}` : ''}`),
  updateModel: (id: string, input: ImageModelInput) => api.put<ImageModel>(`/api/v1/admin/image-agent/models/${id}`, input),
  projects: () => api.get<ImageProject[]>('/api/v1/admin/image-agent/projects'),
  createProject: (input: ImageProjectInput) => api.post<ImageProject>('/api/v1/admin/image-agent/projects', input),
  updateProject: (id: string, input: ImageProjectInput) => api.put<ImageProject>(`/api/v1/admin/image-agent/projects/${id}`, input),
  deleteProject: (id: string) => api.delete<void>(`/api/v1/admin/image-agent/projects/${encodeURIComponent(id)}`),
  promptActions: (projectId?: string) => api.get<ImagePromptAction[]>(`/api/v1/admin/image-agent/prompt-actions${projectId ? `?project_id=${encodeURIComponent(projectId)}` : ''}`),
  createPromptAction: (input: ImagePromptActionInput) => api.post<ImagePromptAction>('/api/v1/admin/image-agent/prompt-actions', input),
  updatePromptAction: (id: string, input: ImagePromptActionInput) => api.put<ImagePromptAction>(`/api/v1/admin/image-agent/prompt-actions/${id}`, input),
  updatePromptActionPreview(id: string, file: File) {
    const body = new FormData()
    body.set('file', file)
    return api.upload<ImagePromptAction>(`/api/v1/admin/image-agent/prompt-actions/${encodeURIComponent(id)}/preview`, body, 'PUT')
  },
  deletePromptActionPreview: (id: string) => api.delete<ImagePromptAction>(`/api/v1/admin/image-agent/prompt-actions/${encodeURIComponent(id)}/preview`),
  deletePromptAction: (id: string) => api.delete(`/api/v1/admin/image-agent/prompt-actions/${id}`),
  backgroundRemovalConfig: () => api.get<PixianBackgroundRemovalConfig>('/api/v1/admin/image-agent/background-removal/config'),
  updateBackgroundRemovalConfig: (input: { enabled: boolean; test_mode: boolean; api_id?: string; api_secret?: string; timeout_seconds: number; concurrency: number; max_pixels: number }) =>
    api.put<PixianBackgroundRemovalConfig>('/api/v1/admin/image-agent/background-removal/config', input),
  testBackgroundRemoval: () => api.post<PixianBackgroundRemovalConfig>('/api/v1/admin/image-agent/background-removal/test'),
  refreshBackgroundRemovalAccount: () => api.post<PixianBackgroundRemovalConfig>('/api/v1/admin/image-agent/background-removal/account/refresh'),
  backgroundRemovalStatistics: () => api.get<BackgroundRemovalStatistics>('/api/v1/admin/image-agent/background-removal/statistics'),
  backgroundRemovalJobs: () => api.get<BackgroundRemovalJob[]>('/api/v1/admin/image-agent/background-removal/jobs?limit=100'),
}
