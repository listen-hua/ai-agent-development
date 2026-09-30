import { describe, expect, it } from 'vitest'
import type { MassageCycleInput } from '@/services/massage'
import { validateMassageCycleInput } from './massageCycle'

function validInput(): MassageCycleInput {
  return {
    service_month: '2026-09',
    title: ' 九月员工按摩 ',
    signup_notice_at: '2026-09-10T09:00:00+08:00',
    signup_deadline: '2026-09-11T18:00:00+08:00',
    audience: { scope: 'all', department_ids: [], user_ids: [], excluded_user_ids: [] },
    sessions: [
      { sequence: 1, starts_at: '2026-09-12T14:00:00+08:00', quota: 50, concurrent_slots: 2 },
      { sequence: 2, starts_at: '2026-09-26T14:00:00+08:00', quota: 50, concurrent_slots: 2 },
    ],
  }
}

describe('validateMassageCycleInput', () => {
  it('normalizes a valid input before submission', () => {
    const result = validateMassageCycleInput(validInput())
    expect(result.error).toBeUndefined()
    expect(result.payload?.title).toBe('九月员工按摩')
    expect(result.payload?.signup_notice_at).toBe('2026-09-10T01:00:00.000Z')
  })

  it('rejects a signup notice that is not before its deadline', () => {
    const input = validInput()
    input.signup_notice_at = input.signup_deadline
    expect(validateMassageCycleInput(input).error).toBe('报名通知时间必须早于报名截止时间')
  })

  it('rejects an empty restricted audience', () => {
    const input = validInput()
    input.audience.scope = 'restricted'
    expect(validateMassageCycleInput(input).error).toBe('指定参与范围时，请至少选择一个部门或人员')
  })
})
