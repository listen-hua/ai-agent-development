import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ImageOperationPanel from './ImageOperationPanel.vue'
import type { ImageFormState } from '@/types/image-agent'

const form = (): ImageFormState => ({
  prompt: '测试', relayId: 'relay', modelId: 'model', projectId: 'project', reversePrompt: false,
  aspectRatio: '1:1', imageSize: '1K', count: 1,
})

function mountPanel(references: Array<Record<string, unknown>> = [], modelValue = form()) {
  return mount(ImageOperationPanel, {
    props: {
      modelValue,
      relays: [{ id: 'relay', name: '中转站' }],
      models: [{ id: 'model', relay_id: 'relay', display_name: '模型', protocol: 'chat_completions', supported_sizes: ['1K'], max_count: 1, supports_reference: true, supports_reverse: false }],
      projects: [{ id: 'project', name: '项目', enabled: true }],
      actions: [],
      references,
    } as never,
    global: {
      stubs: {
        ElIcon: true, ElSelect: true, ElOption: true, ElInput: true, ElSwitch: true, ElButton: true, ElEmpty: true,
        ReferenceImageSlots: true, CartoonStrengthControl: true, ImagePromptActionButton: true,
      },
    },
  })
}

describe('ImageOperationPanel original ratio', () => {
  it('disables original ratio without a reference image', () => {
    const button = mountPanel().findAll('.ratio-grid button').find((item) => item.text() === '原图')
    expect(button?.attributes('disabled')).toBeDefined()
  })

  it('selects original ratio when a reference image exists', async () => {
    const wrapper = mountPanel([{ id: 'asset', width: 1000, height: 427 }])
    const button = wrapper.findAll('.ratio-grid button').find((item) => item.text() === '原图')
    await button?.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({ aspectRatio: 'original' })
  })

  it('switches original ratio back to 1:1 when reverse prompt is enabled', async () => {
    const wrapper = mountPanel([{ id: 'asset', width: 1000, height: 427 }], { ...form(), aspectRatio: 'original' })
    const vm = wrapper.vm as unknown as { update: (patch: Partial<ImageFormState>) => void }
    vm.update({ reversePrompt: true })
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({ reversePrompt: true, aspectRatio: '1:1' })
  })
})
