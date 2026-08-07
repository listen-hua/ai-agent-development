import { describe, expect, it } from 'vitest'
import {
  applyLocalCanvasLayout, canvasLayoutFromCanvas, mergeRemoteCanvasWithLocalLayout, sameCanvasLayout,
} from './imageCanvasPersistence'
import type { ImageCanvas, ImageCanvasNode } from '@/types/image-agent'

const node = (overrides: Partial<ImageCanvasNode> = {}): ImageCanvasNode => ({
  id: 'node-1', canvas_id: 'canvas-1', output_index: 0, status: 'pending',
  x: 10, y: 20, width: 360, height: 360, z_index: 1,
  created_at: '2026-08-04T00:00:00Z', updated_at: '2026-08-04T00:00:00Z',
  ...overrides,
})

const canvas = (overrides: Partial<ImageCanvas> = {}): ImageCanvas => ({
  id: 'canvas-1', user_id: 'user-1', project_id: 'project-1', name: '测试画布', node_count: 1,
  viewport: { x: 0, y: 0, zoom: 1 }, version: 1, nodes: [node()],
  created_at: '2026-08-04T00:00:00Z', updated_at: '2026-08-04T00:00:00Z',
  ...overrides,
})

describe('image canvas persistence helpers', () => {
  it('treats an omitted nodes field from an empty canvas as an empty array', () => {
    const empty = canvas({ nodes: undefined as unknown as ImageCanvasNode[] })
    expect(canvasLayoutFromCanvas(empty).nodes).toEqual([])
    expect(mergeRemoteCanvasWithLocalLayout(empty, empty).nodes).toEqual([])
  })

  it('ignores insignificant floating point movement', () => {
    const original = canvasLayoutFromCanvas(canvas())
    const almostEqual = canvasLayoutFromCanvas(canvas({
      viewport: { x: 0.005, y: -0.005, zoom: 1.005 },
      nodes: [node({ x: 10.005, y: 19.995 })],
    }))
    expect(sameCanvasLayout(original, almostEqual)).toBe(true)
    almostEqual.nodes[0].x = 11
    expect(sameCanvasLayout(original, almostEqual)).toBe(false)
  })

  it('applies local coordinates without discarding node metadata', () => {
    const current = canvas({ nodes: [node({ status: 'ready', asset_id: 'asset-1' })] })
    const updated = applyLocalCanvasLayout(current, {
      viewport: { x: 5, y: 6, zoom: 1.2 },
      nodes: [node({ x: 100, y: 200 })],
    })
    expect(updated.nodes[0]).toMatchObject({ x: 100, y: 200, status: 'ready', asset_id: 'asset-1' })
    expect(updated.viewport).toEqual({ x: 5, y: 6, zoom: 1.2 })
  })

  it('keeps local positions while accepting remote job status and new nodes', () => {
    const local = canvas({ nodes: [node({ x: 700, y: 800 })] })
    const remote = canvas({
      version: 4,
      nodes: [
        node({ status: 'ready', asset_id: 'asset-1', x: 10, y: 20 }),
        node({ id: 'node-2', x: 400, y: 500, status: 'pending' }),
      ],
    })
    const merged = mergeRemoteCanvasWithLocalLayout(remote, local)
    expect(merged.version).toBe(4)
    expect(merged.nodes[0]).toMatchObject({ x: 700, y: 800, status: 'ready', asset_id: 'asset-1' })
    expect(merged.nodes[1]).toMatchObject({ id: 'node-2', x: 400, y: 500 })
  })
})
