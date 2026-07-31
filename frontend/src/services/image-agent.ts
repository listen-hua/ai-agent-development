import { api, resolveApiURL } from './api'
import type {
  ImageAgentOptions, ImageAsset, ImageCanvas, ImageCanvasNode, ImageJob, ImageProjectInput,
  ImagePromptAction, ImagePromptActionInput, ImageRelay, ImageRelayInput, ImageModel, ImageModelInput,
  ImageProject, ImageRatio, ImageSize, ImageCount, ImageViewport,
} from '@/types/image-agent'

export const imageAssetURL = (id: string) => resolveApiURL(`/api/v1/image-agent/assets/${id}/content`)
export const imagePromptActionPreviewURL = (id: string, version?: string) => {
  const query = version ? `?v=${encodeURIComponent(version)}` : ''
  return resolveApiURL(`/api/v1/image-agent/prompt-actions/${encodeURIComponent(id)}/preview${query}`)
}

export const imageAgentService = {
  options: (projectId?: string) => api.get<ImageAgentOptions>(`/api/v1/image-agent/options${projectId ? `?project_id=${encodeURIComponent(projectId)}` : ''}`),
  canvas: (projectId: string) => api.get<ImageCanvas>(`/api/v1/image-agent/projects/${projectId}/canvas`),
  saveCanvas: (projectId: string, input: { viewport: ImageViewport; nodes: ImageCanvasNode[]; version: number }) =>
    api.patch<ImageCanvas>(`/api/v1/image-agent/projects/${projectId}/canvas`, input),
  deleteCanvasNode: (projectId: string, nodeId: string, version: number) =>
    api.delete<ImageCanvas>(
      `/api/v1/image-agent/projects/${encodeURIComponent(projectId)}/canvas/nodes/${encodeURIComponent(nodeId)}?version=${version}`,
    ),
  importCanvasAsset(projectId: string, file: File, input: { x: number; y: number; version: number; origin: 'paste' | 'drop' }) {
    const body = new FormData()
    body.set('file', file)
    body.set('x', String(input.x))
    body.set('y', String(input.y))
    body.set('version', String(input.version))
    body.set('origin', input.origin)
    return api.upload<ImageCanvas>(`/api/v1/image-agent/projects/${projectId}/canvas/imports`, body)
  },
  uploadAsset(file: File, projectId: string) {
    const body = new FormData()
    body.set('file', file)
    body.set('project_id', projectId)
    return api.upload<ImageAsset>('/api/v1/image-agent/assets', body)
  },
  createJob: (input: {
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
}
