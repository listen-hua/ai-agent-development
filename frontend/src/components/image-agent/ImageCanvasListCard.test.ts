import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ImageCanvasListCard from './ImageCanvasListCard.vue'
import type { ImageCanvas } from '@/types/image-agent'

vi.mock('@/services/image-agent', () => ({ imageAssetURL: (id: string) => `/assets/${id}` }))

const canvas = (previewAssetId?: string): ImageCanvas => ({
  id: 'canvas-1', user_id: 'user-1', name: '宣传画布', node_count: 2,
  preview_asset_id: previewAssetId, viewport: { x: 0, y: 0, zoom: 1 }, version: 1, nodes: [],
  created_at: '2026-08-07T00:00:00Z', updated_at: '2026-08-07T01:00:00Z',
})

describe('ImageCanvasListCard', () => {
  it('renders the first canvas image as its preview', () => {
    const wrapper = mount(ImageCanvasListCard, { props: { canvas: canvas('asset-first') } })
    expect(wrapper.get('.canvas-preview img').attributes('src')).toBe('/assets/asset-first')
    expect(wrapper.get('.canvas-preview span').text()).toBe('2')
  })

  it('falls back to the placeholder when preview loading fails', async () => {
    const wrapper = mount(ImageCanvasListCard, { props: { canvas: canvas('asset-first') } })
    await wrapper.get('.canvas-preview img').trigger('error')
    expect(wrapper.find('.canvas-preview img').exists()).toBe(false)
  })
})
