import ElementPlus from 'element-plus'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import MeetingBookingCard from './MeetingBookingCard.vue'
import type { MeetingBookingAction } from '@/types/domain'

function action(status: MeetingBookingAction['status'], selectedOptionId?: string): MeetingBookingAction {
  return {
    id: 'action-1',
    user_id: 'user-1',
    intent: 'create',
    title: '周会',
    attendees: [],
    capacity: 4,
    options: [
      { id: 'option-1', room_id: 'room-1', room_name: '1号会议室', capacity: 4, start_at: '2026-07-24T08:00:00Z', end_at: '2026-07-24T08:30:00Z' },
      { id: 'option-2', room_id: 'room-2', room_name: '2号会议室', capacity: 6, start_at: '2026-07-24T08:00:00Z', end_at: '2026-07-24T08:30:00Z' },
    ],
    selected_option_id: selectedOptionId,
    status,
    source_channel: 'h5',
    expires_at: '2026-07-24T08:15:00Z',
    created_at: '2026-07-24T08:00:00Z',
  }
}

describe('MeetingBookingCard', () => {
  it('shows all candidates while the action is pending', () => {
    const wrapper = mount(MeetingBookingCard, { props: { action: action('pending') }, global: { plugins: [ElementPlus] } })
    const options = wrapper.findAll('.meeting-options .el-radio')
    expect(options).toHaveLength(2)
    expect(options[0].classes()).not.toContain('is-disabled')
  })

  it('locks and only shows the confirmed meeting room', async () => {
    const wrapper = mount(MeetingBookingCard, { props: { action: action('pending') }, global: { plugins: [ElementPlus] } })
    await wrapper.setProps({ action: action('confirmed', 'option-2') })

    const options = wrapper.findAll('.meeting-options .el-radio')
    expect(options).toHaveLength(1)
    expect(options[0].text()).toContain('2号会议室')
    expect(options[0].text()).not.toContain('1号会议室')
    expect(options[0].classes()).toContain('is-disabled')
    expect(wrapper.text()).toContain('会议室已确认，不能再更换')
  })

  it('keeps the user selection while confirmation is processing', async () => {
    const wrapper = mount(MeetingBookingCard, { props: { action: action('pending') }, global: { plugins: [ElementPlus] } })
    await wrapper.findAll('input[type="radio"]')[1].setValue(true)
    await wrapper.setProps({ action: action('processing') })

    const options = wrapper.findAll('.meeting-options .el-radio')
    expect(options).toHaveLength(1)
    expect(options[0].text()).toContain('2号会议室')
    expect(options[0].classes()).toContain('is-disabled')
    expect(wrapper.text()).toContain('正在锁定所选会议室，请稍候')
  })
})
