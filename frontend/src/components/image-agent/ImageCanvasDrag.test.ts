import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ImageCanvasNode from './ImageCanvasNode.vue'
import ReferenceImageSlots from './ReferenceImageSlots.vue'

function transfer(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial))
  return {
    effectAllowed: 'none',
    dropEffect: 'none',
    files: [] as unknown as FileList,
    getData: (type: string) => values.get(type) || '',
    setData: (type: string, value: string) => values.set(type, value),
  }
}

describe('image canvas reference drag and drop', () => {
  it('writes both the custom and browser-compatible drag payloads', async () => {
    const dataTransfer = transfer()
    const wrapper = mount(ImageCanvasNode, {
      props: { data: { assetId: 'asset-1', status: 'ready' } },
      global: { stubs: { ElIcon: true } },
    })

    await wrapper.get('.node-reference-handle').trigger('dragstart', { dataTransfer })

    expect(dataTransfer.effectAllowed).toBe('copy')
    expect(dataTransfer.getData('application/x-image-asset-id')).toBe('asset-1')
    expect(dataTransfer.getData('text/plain')).toBe('image-asset:asset-1')
  })

  it('accepts a canvas asset in a specific empty reference slot', async () => {
    const wrapper = mount(ReferenceImageSlots, {
      props: { assets: [] },
      global: { stubs: { ElIcon: true } },
    })

    await wrapper.get('.reference-empty').trigger('drop', {
      dataTransfer: transfer({ 'application/x-image-asset-id': 'asset-2' }),
    })

    expect(wrapper.emitted('canvas-drop')).toEqual([['asset-2', 0]])
  })

  it('uses the text payload fallback when the embedded browser strips custom MIME types', async () => {
    const wrapper = mount(ReferenceImageSlots, {
      props: { assets: [] },
      global: { stubs: { ElIcon: true } },
    })

    await wrapper.get('.reference-slots').trigger('drop', {
      dataTransfer: transfer({ 'text/plain': 'image-asset:asset-3' }),
    })

    expect(wrapper.emitted('canvas-drop')).toEqual([['asset-3', undefined]])
  })
})
