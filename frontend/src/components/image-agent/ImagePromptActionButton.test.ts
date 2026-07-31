import ElementPlus from 'element-plus'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ImagePromptActionButton from './ImagePromptActionButton.vue'
import type { ImagePromptAction } from '@/types/image-agent'

function action(hasPreview: boolean): ImagePromptAction {
  return {
    id: 'action-1',
    action_key: 'cartoonize',
    name: '卡通化',
    prompt_template: 'cartoon',
    enabled: true,
    sort_order: 1,
    has_preview: hasPreview,
    preview_mime_type: hasPreview ? 'image/png' : undefined,
    created_at: '2026-07-31T00:00:00Z',
    updated_at: '2026-07-31T00:00:00Z',
  }
}

describe('ImagePromptActionButton', () => {
  it('adds a hover preview only when the action has a preview image', () => {
    const withPreview = mount(ImagePromptActionButton, {
      props: { action: action(true) },
      global: { plugins: [ElementPlus] },
    })
    const withoutPreview = mount(ImagePromptActionButton, {
      props: { action: action(false) },
      global: { plugins: [ElementPlus] },
    })

    expect(withPreview.findComponent({ name: 'ElPopover' }).exists()).toBe(true)
    expect(withoutPreview.findComponent({ name: 'ElPopover' }).exists()).toBe(false)
  })

  it('keeps the original button click behavior', async () => {
    const value = action(true)
    const wrapper = mount(ImagePromptActionButton, {
      props: { action: value },
      global: { plugins: [ElementPlus] },
    })

    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('select')?.[0]).toEqual([value])
  })
})
