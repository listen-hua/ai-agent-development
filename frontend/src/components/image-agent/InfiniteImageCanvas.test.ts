import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import InfiniteImageCanvas from './InfiniteImageCanvas.vue'
import type { ImageCanvas } from '@/types/image-agent'

const flow = vi.hoisted(() => ({
  setViewport: vi.fn(),
  getViewport: vi.fn(() => ({ x: 0, y: 0, zoom: 1 })),
  setCenter: vi.fn(),
  screenToFlowCoordinate: vi.fn(({ x, y }: { x: number; y: number }) => ({ x, y })),
  updateNode: vi.fn(),
  selectedNodes: { value: [] as Array<{ id: string }> },
}))

vi.mock('@vue-flow/core', async () => {
  const { defineComponent } = await import('vue')
  return {
    SelectionMode: { Full: 'full', Partial: 'partial' },
    VueFlow: defineComponent({
      name: 'VueFlow',
      props: ['selectionKeyCode', 'selectionMode', 'panOnDrag', 'multiSelectionKeyCode'],
      template: '<div class="vue-flow-stub"><slot /></div>',
    }),
    useVueFlow: () => ({
      setViewport: flow.setViewport,
      getViewport: flow.getViewport,
      setCenter: flow.setCenter,
      screenToFlowCoordinate: flow.screenToFlowCoordinate,
      getSelectedNodes: flow.selectedNodes,
      updateNode: flow.updateNode,
    }),
  }
})

function canvas(id: string, x: number, version = 1): ImageCanvas {
  return {
    id, name: `Canvas ${id}`, node_count: 0,
    user_id: 'user-1',
    project_id: `project-${id}`,
    viewport: { x, y: 20, zoom: 1 },
    version,
    created_at: '2026-07-29T00:00:00Z',
    updated_at: '2026-07-29T00:00:00Z',
    nodes: [],
  }
}

describe('InfiniteImageCanvas viewport synchronization', () => {
  beforeEach(() => { flow.selectedNodes.value = [] })

  it('does not restore an old server viewport when the same canvas refreshes', async () => {
    flow.setViewport.mockClear()
    const wrapper = mount(InfiniteImageCanvas, {
      props: { canvas: canvas('one', 10) },
      global: {
        stubs: {
          Background: true,
          Controls: true,
          MiniMap: true,
          ImageCanvasNode: true,
          CanvasAlignmentGuides: true,
        },
        directives: { loading: () => undefined },
      },
    })
    await nextTick()
    expect(flow.setViewport).toHaveBeenLastCalledWith({ x: 10, y: 20, zoom: 1 }, { duration: 0 })

    flow.setViewport.mockClear()
    await wrapper.setProps({ canvas: canvas('one', -900, 2) })
    await nextTick()

    expect(flow.setViewport).not.toHaveBeenCalled()
  })

  it('restores the saved viewport after switching to a different canvas', async () => {
    flow.setViewport.mockClear()
    const wrapper = mount(InfiniteImageCanvas, {
      props: { canvas: canvas('one', 10) },
      global: {
        stubs: {
          Background: true,
          Controls: true,
          MiniMap: true,
          ImageCanvasNode: true,
          CanvasAlignmentGuides: true,
        },
        directives: { loading: () => undefined },
      },
    })
    await nextTick()
    flow.setViewport.mockClear()

    await wrapper.setProps({ canvas: canvas('two', 240) })
    await nextTick()

    expect(flow.setViewport).toHaveBeenLastCalledWith({ x: 240, y: 20, zoom: 1 }, { duration: 0 })
  })

  it('uses left drag for full containment selection and Space for panning', async () => {
    const wrapper = mount(InfiniteImageCanvas, {
      props: { canvas: canvas('one', 10) },
      global: {
        stubs: { Background: true, Controls: true, MiniMap: true, ImageCanvasNode: true, CanvasAlignmentGuides: true },
        directives: { loading: () => undefined },
      },
    })
    const vueFlow = wrapper.findComponent({ name: 'VueFlow' })
    expect(vueFlow.props('selectionKeyCode')).toBe(true)
    expect(vueFlow.props('selectionMode')).toBe('full')
    expect(vueFlow.props('panOnDrag')).toEqual([1])

    window.dispatchEvent(new KeyboardEvent('keydown', { code: 'Space' }))
    await nextTick()
    expect(vueFlow.props('panOnDrag')).toBe(true)

    window.dispatchEvent(new Event('blur'))
    await nextTick()
    expect(vueFlow.props('selectionKeyCode')).toBe(true)
    expect(vueFlow.props('panOnDrag')).toEqual([1])
    wrapper.unmount()
  })

  it('shows the current selected image count', () => {
    flow.selectedNodes.value = [{ id: 'node-1' }, { id: 'node-2' }]
    const wrapper = mount(InfiniteImageCanvas, {
      props: { canvas: canvas('one', 10) },
      global: {
        stubs: { Background: true, Controls: true, MiniMap: true, ImageCanvasNode: true, CanvasAlignmentGuides: true },
        directives: { loading: () => undefined },
      },
    })
    expect(wrapper.get('.selection-count').text()).toBe('已选择 2 张图片')
    wrapper.unmount()
  })
})
