import { beforeEach, describe, expect, it, vi } from 'vitest'
import { imageAgentService, normalizeImageAgentOptions, normalizeImageCanvas } from './image-agent'
import type { ImageCanvas } from '@/types/image-agent'

const api = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn(), patch: vi.fn(), delete: vi.fn(), upload: vi.fn() }))
vi.mock('./api', () => ({ api, resolveApiURL: (value: string) => value }))

describe('image agent response normalization', () => {
  it('supplies empty collections when an older or empty API response omits them', () => {
    expect(normalizeImageAgentOptions({})).toEqual({ relays: [], models: [], projects: [], prompt_actions: [] })
    const canvas = normalizeImageCanvas({
      id: 'canvas-1', user_id: 'user-1', project_id: 'project-1', version: 1,
      viewport: { x: 0, y: 0, zoom: 1 }, nodes: undefined,
      created_at: '', updated_at: '',
    } as unknown as ImageCanvas)
    expect(canvas.nodes).toEqual([])
  })
})

describe('image canvas API', () => {
  beforeEach(() => Object.values(api).forEach((mock) => mock.mockReset()))

  it('uses canvas-scoped endpoints for listing, saving, deleting, and restoring', async () => {
    const canvas = {
      id: 'canvas-1', user_id: 'user-1', name: '宣传图', node_count: 0,
      viewport: { x: 0, y: 0, zoom: 1 }, version: 1, nodes: [], created_at: '', updated_at: '',
    } satisfies ImageCanvas
    api.get.mockResolvedValueOnce([canvas]).mockResolvedValueOnce(canvas)
    api.patch.mockResolvedValue(canvas)
    api.delete.mockResolvedValue(undefined)
    api.post.mockResolvedValue(canvas)

    await imageAgentService.canvases()
    await imageAgentService.canvasById(canvas.id)
    await imageAgentService.saveCanvasById(canvas.id, { viewport: canvas.viewport, nodes: [], version: 1 })
    await imageAgentService.removeCanvas(canvas.id)
    await imageAgentService.restoreCanvas(canvas.id)

    expect(api.get).toHaveBeenNthCalledWith(1, '/api/v1/image-agent/canvases')
    expect(api.get).toHaveBeenNthCalledWith(2, '/api/v1/image-agent/canvases/canvas-1')
    expect(api.patch).toHaveBeenCalledWith('/api/v1/image-agent/canvases/canvas-1', expect.objectContaining({ version: 1 }))
    expect(api.delete).toHaveBeenCalledWith('/api/v1/image-agent/canvases/canvas-1')
    expect(api.post).toHaveBeenCalledWith('/api/v1/image-agent/canvases/canvas-1/restore')
  })

  it('loads only the selected project prompt actions', async () => {
    api.get.mockResolvedValue([])
    await imageAgentService.promptActions('project/one')
    expect(api.get).toHaveBeenCalledWith('/api/v1/image-agent/prompt-actions?project_id=project%2Fone')
  })
})
