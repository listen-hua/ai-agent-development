import type { ReminderSchedule, ReminderStatus } from '@/types/domain'

const weekdayNames = ['周一', '周二', '周三', '周四', '周五', '周六', '周日']

export function formatReminderSchedule(schedule: ReminderSchedule) {
  if (schedule.type === 'once') return schedule.once_at ? new Date(schedule.once_at).toLocaleString('zh-CN', { hour12: false }) : '-'
  if (schedule.type === 'daily') return `每天 ${schedule.local_time}`
  if (schedule.type === 'workday') return `每个公司工作日 ${schedule.local_time}`
  return `每${(schedule.weekdays || []).map((day) => weekdayNames[day - 1]).join('、')} ${schedule.local_time}`
}

export function reminderStatusText(status: ReminderStatus) {
  return ({ active: '生效中', paused: '已暂停', completed: '已完成', cancelled: '已删除', failed: '发送失败' } as const)[status]
}

export function reminderStatusType(status: ReminderStatus): 'success' | 'warning' | 'info' | 'danger' {
  return status === 'active' ? 'success' : status === 'failed' ? 'danger' : status === 'paused' ? 'warning' : 'info'
}
