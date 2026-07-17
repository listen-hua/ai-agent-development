import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ChatComposer from './ChatComposer.vue'

describe('ChatComposer', () => {
  it('submits trimmed content with Enter', async () => {
    const wrapper = mount(ChatComposer, { props: { disabled: false }, global: { stubs: { ElButton: true } } })
    const textarea = wrapper.get('textarea')
    await textarea.setValue('  年假怎么申请？  ')
    await textarea.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('send')?.[0]).toEqual(['年假怎么申请？'])
  })

  it('does not submit while disabled', async () => {
    const wrapper = mount(ChatComposer, { props: { disabled: true }, global: { stubs: { ElButton: true } } })
    const textarea = wrapper.get('textarea')
    await textarea.setValue('测试')
    await textarea.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('send')).toBeUndefined()
  })
})
