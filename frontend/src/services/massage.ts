import { api, resolveApiURL } from './api'
import type { MassageCycle, MassageEnrollment, MassageMe, MassageSession, MassageStatistic, MassageCall } from '@/types/domain'

export interface MassageCycleInput {
  service_month: string
  title: string
  signup_notice_at: string
  signup_deadline: string
  audience: { scope: 'all' | 'restricted'; department_ids: string[]; user_ids: string[]; excluded_user_ids: string[] }
  sessions: Array<{ id?: string; sequence: 1 | 2; starts_at: string; quota: number; concurrent_slots: number }>
}

export const massageService = {
  me: () => api.get<MassageMe[]>('/api/v1/massage/me'),
  respond: (cycleId: string, action: 'enroll' | 'decline' | 'withdraw') => api.post<MassageEnrollment>(`/api/v1/massage/cycles/${cycleId}/response`, { action }),
  list: () => api.get<MassageCycle[]>('/api/v1/admin/massage/cycles'),
  get: (id: string) => api.get<MassageCycle>(`/api/v1/admin/massage/cycles/${id}`),
  create: (input: MassageCycleInput) => api.post<MassageCycle>('/api/v1/admin/massage/cycles', input),
  update: (id: string, input: MassageCycleInput) => api.put<MassageCycle>(`/api/v1/admin/massage/cycles/${id}`, input),
  remove: (id: string) => api.delete<void>(`/api/v1/admin/massage/cycles/${id}`),
  publish: (id: string) => api.post<MassageCycle>(`/api/v1/admin/massage/cycles/${id}/publish`),
  resend: (id: string) => api.post<{ count: number }>(`/api/v1/admin/massage/cycles/${id}/signup-reminders/resend`),
  sessionAction: (id: string, action: 'start' | 'pause' | 'resume' | 'close', reason = '') => api.post<MassageSession>(`/api/v1/admin/massage/sessions/${id}/${action}`, { reason }),
  callAction: (id: string, action: 'complete' | 'no-show') => api.post<MassageCall>(`/api/v1/admin/massage/calls/${id}/${action}`),
  statistics: (id: string) => api.get<MassageStatistic>(`/api/v1/admin/massage/cycles/${id}/statistics`),
  csvURL: (id: string) => resolveApiURL(`/api/v1/admin/massage/cycles/${id}/statistics.csv`),
}
