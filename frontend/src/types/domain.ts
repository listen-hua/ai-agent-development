export type Role = 'employee' | 'knowledge_admin' | 'notification_admin' | 'auditor' | 'super_admin'

export interface User { id: string; feishu_open_id: string; name: string; avatar_url: string; department_ids: string[]; job_title: string; job_level_id: string; job_family_id: string; employee_type: number; status: string; organization_synced_at?: string; roles: Role[] }
export interface ACLRule { department_ids?: string[]; job_titles?: string[]; job_level_ids?: string[]; job_family_ids?: string[]; employee_types?: number[]; user_ids?: string[] }
export interface ACL { scope: 'all' | 'restricted'; rules?: ACLRule[]; department_ids?: string[]; role_names?: Role[]; user_ids?: string[] }
export interface DirectoryOptions { departments: Array<{ id: string; name: string }>; job_titles: string[]; users: Array<{ id: string; name: string; department_ids: string[]; job_title: string }> }
export interface Citation { id: string; document_id: string; version_id: string; title: string; version: string; heading: string; page: number; excerpt: string; source_url?: string }
export type ReminderScheduleType = 'once' | 'daily' | 'workday' | 'weekly'
export interface ReminderSchedule { type: ReminderScheduleType; timezone: 'Asia/Shanghai'; local_time?: string; weekdays?: number[]; once_at?: string }
export type ReminderStatus = 'active' | 'paused' | 'completed' | 'cancelled' | 'failed'
export interface Reminder { id: string; user_id: string; content: string; status: ReminderStatus; schedule: ReminderSchedule; next_fire_at?: string; last_fired_at?: string; last_error?: string; source_channel: 'h5' | 'feishu_bot'; source_conversation_id?: string; version: number; created_at: string; updated_at: string }
export type ReminderActionType = 'create' | 'update' | 'pause' | 'resume' | 'delete'
export interface ReminderActionDraft { id: string; user_id: string; action: ReminderActionType; reminder_id?: string; content?: string; schedule?: ReminderSchedule; status: 'pending' | 'confirmed' | 'cancelled' | 'expired'; source_channel: 'h5' | 'feishu_bot'; source_conversation_id?: string; result_reminder_id?: string; expires_at: string; created_at: string; confirmed_at?: string }
export interface ReminderDelivery { id: string; reminder_id: string; scheduled_for: string; status: 'pending' | 'sending' | 'retry' | 'sent' | 'missed' | 'failed'; attempts: number; next_attempt_at: string; message_id?: string; error?: string; created_at: string; updated_at: string }
export interface WorkdayOverride { date: string; is_workday: boolean; note?: string; updated_by?: string; updated_at: string }
export interface ReminderActionResult { action: ReminderActionDraft; reminder: Reminder }
export interface Message { id: string; conversation_id: string; role: 'user' | 'assistant'; content: string; citations: Citation[]; model?: string; reminder_action?: ReminderActionDraft; created_at: string; pending?: boolean }
export interface Conversation { id: string; user_id: string; title: string; created_at: string; updated_at: string }
export interface RunEvent { type: 'status' | 'delta' | 'citation' | 'done' | 'error'; run_id: string; delta?: string; citation?: Citation; message?: Message; error?: string; metadata?: Record<string, string> }
export interface SourceSyncStats { discovered: number; created: number; updated: number; unchanged: number; unavailable: number; skipped: number; failed: number }
export type KnowledgeSourceType = 'upload' | 'feishu_folder' | 'feishu_wiki'
export type ConnectedKnowledgeSourceType = Exclude<KnowledgeSourceType, 'upload'>
export interface KnowledgeSource { id: string; name: string; type: KnowledgeSourceType; remote_token?: string; default_acl: ACL; sync_status: string; sync_error?: string; last_sync_stats: SourceSyncStats; last_synced_at?: string; created_at: string }
export interface DocumentVersion { id: string; version: string; checksum: string; mime_type: string; status: string; published_at?: string; created_at: string }
export interface KnowledgeDocument { id: string; source_id?: string; title: string; source_url?: string; acl: ACL; status: string; versions: DocumentVersion[]; created_at: string; updated_at: string }
export interface AgentConfig { generation_model: string; embedding_model: string; rerank_model: string; temperature: number; max_output_tokens: number; timeout_seconds: number; retrieval_top_k: number; rerank_top_n: number; score_threshold: number; context_budget: number; system_prompt: string }
export interface AgentConfigVersion { id: string; version: number; status: 'draft' | 'published' | 'archived'; config: AgentConfig; created_by: string; created_at: string; published_at?: string }
export type AgentKind = 'chat' | 'image'
export interface AgentProfile {
  id: string
  agent_key: string
  name: string
  description: string
  kind: AgentKind
  provider: string
  model: string
  enabled: boolean
  has_api_key: boolean
  api_key_hint: string
  credential_source: 'environment' | 'database'
  settings: Record<string, unknown>
  created_at: string
  updated_at: string
}
export interface AgentProfileInput {
  agent_key: string
  name: string
  description: string
  kind: AgentKind
  provider: string
  model: string
  api_key?: string
  enabled: boolean
  settings: Record<string, unknown>
}
export type NotificationRecipientType = 'user' | 'chat'
export interface NotificationImage { image_key: string; name: string; alt: string; preview_url?: string }
export interface NotificationRecipient { type: NotificationRecipientType; id: string; name: string }
export interface NotificationTargetOption { type: NotificationRecipientType; id: string; name: string; avatar_url?: string }
export interface NotificationTargets { users: NotificationTargetOption[]; chats: NotificationTargetOption[]; chat_error?: string }
export interface NotificationCreateInput { title: string; content: string; recipient_type: NotificationRecipientType; recipient_ids: string[]; images: NotificationImage[]; scheduled_at?: string }
export interface NotificationDraft {
  id: string
  title: string
  content: string
  content_format: 'markdown'
  images: NotificationImage[]
  recipient_type: NotificationRecipientType | 'legacy'
  recipients: NotificationRecipient[]
  audience: ACL
  status: 'draft' | 'approved' | 'scheduled' | 'sending' | 'sent' | 'partial' | 'failed' | 'cancelled'
  scheduled_at?: string
  approved_by?: string
  created_by: string
  idempotency_key: string
  recipient_count: number
  last_error?: string
  created_at: string
  updated_at: string
}
export interface AuditEvent { id: string; actor_id: string; actor_name: string; action: string; resource_type: string; resource_id: string; metadata: Record<string, unknown>; created_at: string }
export interface Metrics { questions_today: number; positive_rate: number; no_answer_rate: number; citation_coverage: number; documents_published: number; sync_backlog: number; delivery_success: number }
