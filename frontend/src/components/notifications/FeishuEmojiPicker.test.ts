import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import FeishuEmojiPicker from './FeishuEmojiPicker.vue'

const PopoverStub = defineComponent({
  template: '<div><slot name="reference"/><slot/></div>',
})
const ButtonStub = defineComponent({
  emits: ['click'],
  template: '<button type="button" @click="$emit(\'click\')"><slot/></button>',
})
const InputStub = defineComponent({
  props: ['modelValue'],
  emits: ['update:modelValue'],
  template: '<input class="picker-search" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)">',
})

function mountPicker() {
  return mount(FeishuEmojiPicker, {
    global: {
      stubs: { ElPopover: PopoverStub, ElButton: ButtonStub, ElInput: InputStub, ElEmpty: true },
    },
  })
}

describe('FeishuEmojiPicker', () => {
  it('emits the selected official emoji', async () => {
    const wrapper = mountPicker()
    const buttons = wrapper.findAll('.emoji-grid button')
    expect(buttons).toHaveLength(40)
    await buttons[0].trigger('click')
    expect(wrapper.emitted('select')?.[0]?.[0]).toMatchObject({ code: 'OK', name: '好的' })
  })

  it('filters by Chinese name', async () => {
    const wrapper = mountPicker()
    await wrapper.get('.picker-search').setValue('敬礼')
    const buttons = wrapper.findAll('.emoji-grid button')
    expect(buttons).toHaveLength(1)
    expect(buttons[0].attributes('title')).toContain(':SALUTE:')
  })
})
