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
  it('shows the actual image resolution after the image loads', async () => {
    const wrapper = mount(ImageCanvasNode, {
      props: { data: { assetId: 'asset-resolution', status: 'ready' } },
      global: { stubs: { ElIcon: true } },
    })
    const image = wrapper.get('img')
    Object.defineProperties(image.element, {
      naturalWidth: { value: 1024 },
      naturalHeight: { value: 768 },
    })

    await image.trigger('load')

		expect(wrapper.get('.node-resolution').text()).toBe('实际：1024 × 768')
    expect(wrapper.get('.canvas-image-node').attributes('tabindex')).toBe('0')
	})

	it('shows the requested tier and downgrade warning with the actual resolution', async () => {
		const wrapper = mount(ImageCanvasNode, {
			props: { data: { assetId: 'asset-low-resolution', status: 'ready', requestedSize: '2K', resolutionWarning: '中转站未执行2K档位' } },
		})
		const image = wrapper.get('img')
		Object.defineProperty(image.element, 'naturalWidth', { value: 1254 })
		Object.defineProperty(image.element, 'naturalHeight', { value: 1254 })
		await image.trigger('load')
		expect(wrapper.get('.node-resolution').text()).toContain('请求：2K')
		expect(wrapper.get('.node-resolution').text()).toContain('实际：1254 × 1254')
		expect(wrapper.get('.node-resolution').text()).toContain('分辨率已降级')
	})

  it('shows the generation relay and model only for a ready AI image', () => {
    const wrapper = mount(ImageCanvasNode, {
      props: {
        data: {
          assetId: 'asset-generated',
          status: 'ready',
          generationRelayName: 'Comfly',
          generationModelName: 'GPT Image 2',
          generationModelKey: 'gpt-image-2',
        },
      },
    })

    const badge = wrapper.get('.generation-source-badge')
    expect(badge.text()).toContain('中转站Comfly')
    expect(badge.text()).toContain('模型GPT Image 2')
    expect(badge.text()).toContain('模型 IDgpt-image-2')
  })

  it('marks a Pixian result while preserving its original model source', () => {
    const wrapper = mount(ImageCanvasNode, {
      props: {
        data: {
          assetId: 'asset-cutout',
          status: 'ready',
          backgroundRemoval: true,
          generationRelayName: 'XGAPI',
          generationModelName: 'Gemini Image',
          generationModelKey: 'gemini-3.1-flash-image-preview',
        },
      },
    })

    expect(wrapper.get('.generation-source-badge').text()).toContain('Pixian 智能抠图')
    expect(wrapper.get('.generation-source-badge').text()).toContain('XGAPI')
  })

  it('does not show an AI source for an ordinary uploaded image', () => {
    const wrapper = mount(ImageCanvasNode, {
      props: { data: { assetId: 'asset-upload', status: 'ready' } },
    })

    expect(wrapper.find('.generation-source-badge').exists()).toBe(false)
  })

  it('emits a delete command from the canvas node action', async () => {
    const wrapper = mount(ImageCanvasNode, {
      props: { data: { assetId: 'asset-delete', status: 'ready' } },
      global: { stubs: { ElIcon: true } },
    })

    await wrapper.get('.node-delete-button').trigger('click')

    expect(wrapper.emitted('delete')).toHaveLength(1)
  })

  it('does not expose deletion for a read-only project', () => {
    const wrapper = mount(ImageCanvasNode, {
      props: { data: { assetId: 'asset-readonly', status: 'ready' }, readonly: true },
      global: { stubs: { ElIcon: true } },
    })

    expect(wrapper.find('.node-delete-button').exists()).toBe(false)
  })

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
