import { api } from './api'
import type { Reminder, ReminderActionDraft, ReminderActionResult, ReminderDelivery, ReminderSchedule, ReminderActionType } from '@/types/domain'

export const reminderService = {
  list: () => api.get<Reminder[]>('/api/v1/reminders'),
  get: (id: string) => api.get<Reminder>(`/api/v1/reminders/${id}`),
  deliveries: (id: string) => api.get<ReminderDelivery[]>(`/api/v1/reminders/${id}/deliveries`),
  create: (input: { content: string; schedule: ReminderSchedule }) => api.post<ReminderActionDraft>('/api/v1/reminder-actions', input),
  createAction: (id: string, input: { action: Exclude<ReminderActionType, 'create'>; content?: string; schedule?: ReminderSchedule }) => api.post<ReminderActionDraft>(`/api/v1/reminders/${id}/actions`, input),
  confirm: (id: string) => api.post<ReminderActionResult>(`/api/v1/reminder-actions/${id}/confirm`),
  cancelAction: (id: string) => api.post<ReminderActionDraft>(`/api/v1/reminder-actions/${id}/cancel`),
}
