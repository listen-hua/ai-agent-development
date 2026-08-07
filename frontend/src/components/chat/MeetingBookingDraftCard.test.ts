import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MeetingBookingDraftCard from './MeetingBookingDraftCard.vue'
import { meetingService } from '@/services/meeting'
import { useAuthStore } from '@/stores/auth'
import type { MeetingBookingDraft } from '@/types/domain'

vi.mock('@/services/meeting', () => ({ meetingService: { searchAttendees: vi.fn(), updateDraft: vi.fn() } }))

function draft(): MeetingBookingDraft {
  return {
    id: 'draft-1', user_id: 'user-1', conversation_id: 'conversation-1', channel: 'h5', intent: 'create',
    slots: { date: '2026-08-06', hour: 15, minute: 0, duration_minutes: 30, attendee_user_ids: [], attendees_confirmed: false },
    missing_fields: ['title', 'attendees'], status: 'collecting', version: 1,
    expires_at: '2099-08-06T07:15:00Z', created_at: '2026-08-06T07:00:00Z', updated_at: '2026-08-06T07:00:00Z',
  }
}

beforeEach(() => {
  setActivePinia(createPinia())
  useAuthStore().acceptResolvedUser({ id: 'user-1', feishu_open_id: 'ou_user', name: '申请人', avatar_url: '', department_ids: [], job_title: '', job_level_id: '', job_family_id: '', employee_type: 0, status: 'active', roles: ['employee'], permissions: ['agent_use'], permission_sources: { iam: [], local_allow: ['agent_use'], local_deny: [] } })
  vi.mocked(meetingService.searchAttendees).mockResolvedValue([])
  vi.mocked(meetingService.updateDraft).mockResolvedValue({ draft: { ...draft(), status: 'completed', missing_fields: [], slots: { ...draft().slots, title: '产品周报评审', attendees_confirmed: true } } })
})

describe('MeetingBookingDraftCard', () => {
  it('requires a topic and submits an explicit self-only attendee choice', async () => {
    const wrapper = mount(MeetingBookingDraftCard, { props: { draft: draft() }, global: { plugins: [ElementPlus] } })
    await wrapper.find('.el-input__inner').setValue('产品周报评审')
    const selfButton = wrapper.findAll('button').find((button) => button.text().includes('仅自己参会'))
    await selfButton?.trigger('click')
    await vi.waitFor(() => expect(meetingService.updateDraft).toHaveBeenCalledWith('draft-1', {
      title: '产品周报评审', attendee_user_ids: [], attendees_confirmed: true, version: 1,
    }))
  })
})
