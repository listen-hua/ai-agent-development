import { api } from './api'
import type { ACL, AgentConfig, AgentConfigVersion, AgentProfile, AgentProfileInput, AuditEvent, ConnectedKnowledgeSourceType, DirectoryOptions, KnowledgeDocument, KnowledgeSource, Metrics, NotificationCreateInput, NotificationDraft, NotificationImage, NotificationTargets, PermissionKey, User, WorkdayOverride } from '@/types/domain'

export const knowledgeService = {
  listSources: () => api.get<KnowledgeSource[]>('/api/v1/admin/knowledge/sources'),
  createSource: (input: { name: string; type: ConnectedKnowledgeSourceType; remote_token: string; default_acl: ACL }) => api.post<KnowledgeSource>('/api/v1/admin/knowledge/sources', input),
  syncSource: (id: string) => api.post<{ status: string }>(`/api/v1/admin/knowledge/sources/${id}/sync`),
  listDocuments: () => api.get<KnowledgeDocument[]>('/api/v1/admin/knowledge/documents'),
  upload(file: File, acl: ACL) { const data = new FormData(); data.set('file', file); data.set('acl', JSON.stringify(acl)); return api.upload<KnowledgeDocument>('/api/v1/admin/knowledge/documents/upload', data) },
  publish: (id: string) => api.post<KnowledgeDocument>(`/api/v1/admin/knowledge/documents/${id}/publish`),
  updateACL: (id: string, acl: ACL) => api.put<KnowledgeDocument>(`/api/v1/admin/knowledge/documents/${id}/acl`, acl),
}
export const directoryService = {
  options: () => api.get<DirectoryOptions>('/api/v1/admin/directory/options'),
}
export const userAdminService = {
  list: () => api.get<User[]>('/api/v1/admin/users'),
  permissions: (id: string) => api.get<User>(`/api/v1/admin/users/${id}/permissions`),
  updatePermissions: (id: string, input: { allow_keys: PermissionKey[]; deny_keys: PermissionKey[]; version: number; reason: string }) => api.put<User>(`/api/v1/admin/users/${id}/permissions`, input),
  refreshPermissions: (id: string) => api.post<User>(`/api/v1/admin/users/${id}/permissions/refresh`),
  sync: () => api.post<{ succeeded: number; failed: number }>('/api/v1/admin/users/sync'),
}
export const configService = {
  list: () => api.get<AgentConfigVersion[]>('/api/v1/admin/agent/configs'),
  create: (config: AgentConfig) => api.post<AgentConfigVersion>('/api/v1/admin/agent/configs', config),
  publish: (id: string) => api.post<AgentConfigVersion>(`/api/v1/admin/agent/configs/${id}/publish`),
}
export const agentRegistryService = {
  available: () => api.get<AgentProfile[]>('/api/v1/agents'),
  list: () => api.get<AgentProfile[]>('/api/v1/admin/agents'),
  create: (input: AgentProfileInput) => api.post<AgentProfile>('/api/v1/admin/agents', input),
  update: (id: string, input: AgentProfileInput) => api.put<AgentProfile>(`/api/v1/admin/agents/${id}`, input),
}
export const notificationService = {
  list: () => api.get<NotificationDraft[]>('/api/v1/admin/notifications'),
  targets: () => api.get<NotificationTargets>('/api/v1/admin/notifications/targets'),
  create: (input: NotificationCreateInput) => api.post<NotificationDraft>('/api/v1/admin/notifications', input),
  uploadImage(file: File) { const data = new FormData(); data.set('file', file); return api.upload<NotificationImage>('/api/v1/admin/notifications/images', data) },
  aiDraft: (brief: string) => api.post<{ content: string }>('/api/v1/admin/notifications/ai-draft', { brief }),
  approve: (id: string) => api.post<NotificationDraft>(`/api/v1/admin/notifications/${id}/approve`),
  send: (id: string) => api.post<NotificationDraft>(`/api/v1/admin/notifications/${id}/send`),
  cancel: (id: string) => api.post<NotificationDraft>(`/api/v1/admin/notifications/${id}/cancel`),
}
export const workCalendarService = {
  list: (year: number) => api.get<WorkdayOverride[]>(`/api/v1/admin/work-calendar?year=${year}`),
  save: (date: string, input: { is_workday: boolean; note?: string }) => api.put<WorkdayOverride>(`/api/v1/admin/work-calendar/${date}`, input),
  remove: (date: string) => api.delete(`/api/v1/admin/work-calendar/${date}`),
  importCSV(file: File) { const data = new FormData(); data.set('file', file); return api.upload<{ imported: number }>('/api/v1/admin/work-calendar/import', data) },
}
export const auditService = {
  list: () => api.get<AuditEvent[]>('/api/v1/admin/audit'),
  metrics: () => api.get<Metrics>('/api/v1/admin/metrics'),
}
