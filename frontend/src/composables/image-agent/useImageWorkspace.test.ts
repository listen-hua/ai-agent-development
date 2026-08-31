import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'
import { useImageWorkspace } from './useImageWorkspace'
import type { ImageAgentOptions, ImageCanvas } from '@/types/image-agent'

const service = vi.hoisted(() => ({
  options: vi.fn(),
  promptActions: vi.fn(),
  canvasById: vi.fn(),
  saveCanvasById: vi.fn(),
}))

vi.mock('@/services/image-agent', () => ({ imageAgentService: service }))

const options: ImageAgentOptions = {
  relays: [{
    id: 'relay-1', relay_key: 'relay', name: 'Relay', base_url: 'https://example.com', enabled: true,
    timeout_seconds: 60, allowed_output_hosts: [], has_api_key: true, api_key_hint: '***1234',
    created_at: '2026-08-04T00:00:00Z', updated_at: '2026-08-04T00:00:00Z',
  }],
  models: [{
    id: 'model-1', relay_id: 'relay-1', model_id: 'model-1', display_name: 'Model',
    protocol: 'images_generations', enabled: true, supports_reference: false,
    supports_reverse: false, supported_sizes: ['1K'], max_count: 1,
    created_at: '2026-08-04T00:00:00Z', updated_at: '2026-08-04T00:00:00Z',
  }],
  projects: [{
    id: 'project-1', project_key: 'project', name: 'Project', description: '', acl: { scope: 'all' }, enabled: true,
    created_at: '2026-08-04T00:00:00Z', updated_at: '2026-08-04T00:00:00Z',
  }],
  prompt_actions: [],
  background_removal_enabled: false,
}

const initialCanvas = (): ImageCanvas => ({
  id: 'canvas-1', user_id: 'user-1', project_id: 'project-1', name: '测试画布', node_count: 1,
  viewport: { x: 0, y: 0, zoom: 1 }, version: 1,
  nodes: [{
    id: 'node-1', canvas_id: 'canvas-1', output_index: 0, status: 'ready',
    x: 10, y: 20, width: 360, height: 360, z_index: 1,
    created_at: '2026-08-04T00:00:00Z', updated_at: '2026-08-04T00:00:00Z',
  }],
  created_at: '2026-08-04T00:00:00Z', updated_at: '2026-08-04T00:00:00Z',
})

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

function mountWorkspace() {
  let workspace!: ReturnType<typeof useImageWorkspace>
  const wrapper = mount(defineComponent({
    setup() {
      workspace = useImageWorkspace(ref('canvas-1'))
      return () => null
    },
  }))
  return { workspace, wrapper }
}

describe('image workspace canvas save queue', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    service.options.mockReset().mockResolvedValue(options)
    service.promptActions.mockReset().mockResolvedValue([])
    service.canvasById.mockReset().mockResolvedValue(initialCanvas())
    service.saveCanvasById.mockReset()
  })

  afterEach(() => vi.useRealTimers())

  it('ignores an unchanged layout', async () => {
    const { workspace, wrapper } = mountWorkspace()
    await workspace.initialize()
    workspace.scheduleCanvasSave({
      viewport: { ...workspace.canvas.value!.viewport },
      nodes: workspace.canvas.value!.nodes.map((node) => ({ ...node })),
    })
    await vi.advanceTimersByTimeAsync(600)
    expect(service.saveCanvasById).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('keeps the latest drag position when an older save finishes later', async () => {
    const first = deferred<ImageCanvas>()
    service.saveCanvasById
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(async (_projectID, patch) => ({
        ...initialCanvas(), version: 3, viewport: patch.viewport, nodes: patch.nodes,
      }))
    const { workspace, wrapper } = mountWorkspace()
    await workspace.initialize()

    workspace.scheduleCanvasSave({
      viewport: { x: 0, y: 0, zoom: 1 },
      nodes: [{ ...workspace.canvas.value!.nodes[0], x: 100 }],
    })
    await vi.advanceTimersByTimeAsync(550)
    workspace.scheduleCanvasSave({
      viewport: { x: 0, y: 0, zoom: 1 },
      nodes: [{ ...workspace.canvas.value!.nodes[0], x: 300 }],
    })

    first.resolve({ ...initialCanvas(), version: 2 })
    await Promise.resolve()
    await vi.advanceTimersByTimeAsync(150)

    expect(workspace.canvas.value!.nodes[0].x).toBe(300)
    expect(service.saveCanvasById).toHaveBeenCalledTimes(2)
    expect(service.saveCanvasById.mock.calls[1][1].nodes[0].x).toBe(300)
    expect(service.saveCanvasById.mock.calls[1][1].version).toBe(2)
    wrapper.unmount()
  })
})
