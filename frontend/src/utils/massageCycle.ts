import type { MassageCycleInput } from '@/services/massage'

export interface MassageCycleValidationResult {
  payload?: MassageCycleInput
  error?: string
}

function normalizeDateTime(value: string): string | undefined {
  const timestamp = Date.parse(value)
  if (!Number.isFinite(timestamp)) return undefined
  return new Date(timestamp).toISOString()
}

export function validateMassageCycleInput(input: MassageCycleInput): MassageCycleValidationResult {
  const title = input.title.trim()
  if (!input.service_month || !/^\d{4}-\d{2}$/.test(input.service_month)) {
    return { error: '请选择服务月份' }
  }
  if (!title) return { error: '请输入批次名称' }
  if ([...title].length > 100) return { error: '批次名称不能超过 100 个字符' }

  const noticeAt = normalizeDateTime(input.signup_notice_at)
  const deadline = normalizeDateTime(input.signup_deadline)
  const firstStartsAt = normalizeDateTime(input.sessions[0]?.starts_at || '')
  const secondStartsAt = normalizeDateTime(input.sessions[1]?.starts_at || '')
  if (!noticeAt) return { error: '请选择报名通知时间' }
  if (!deadline) return { error: '请选择报名截止时间' }
  if (!firstStartsAt) return { error: '请选择第一场按摩时间' }
  if (!secondStartsAt) return { error: '请选择第二场按摩时间' }
  if (Date.parse(noticeAt) >= Date.parse(deadline)) {
    return { error: '报名通知时间必须早于报名截止时间' }
  }
  if (Date.parse(deadline) > Date.parse(firstStartsAt)) {
    return { error: '报名截止时间不能晚于第一场按摩时间' }
  }
  if (Date.parse(firstStartsAt) >= Date.parse(secondStartsAt)) {
    return { error: '第二场按摩时间必须晚于第一场' }
  }
  if (input.sessions.some(session => session.quota < 1 || session.quota > 10000)) {
    return { error: '每场人数必须在 1–10000 之间' }
  }
  if (input.sessions.some(session => session.concurrent_slots < 1 || session.concurrent_slots > 20)) {
    return { error: '并行服务人数必须在 1–20 之间' }
  }
  if (input.audience.scope === 'restricted'
    && input.audience.department_ids.length === 0
    && input.audience.user_ids.length === 0) {
    return { error: '指定参与范围时，请至少选择一个部门或人员' }
  }

  return {
    payload: {
      ...input,
      title,
      signup_notice_at: noticeAt,
      signup_deadline: deadline,
      audience: {
        ...input.audience,
        department_ids: [...input.audience.department_ids],
        user_ids: [...input.audience.user_ids],
        excluded_user_ids: [...input.audience.excluded_user_ids],
      },
      sessions: input.sessions.map((session, index) => ({
        ...session,
        sequence: (index + 1) as 1 | 2,
        starts_at: index === 0 ? firstStartsAt : secondStartsAt,
      })),
    },
  }
}
