import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import type { PermissionKey } from '@/types/domain'
import { installChunkLoadRecovery } from './chunkRecovery'

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { public: true } },
  {
    path: '/', component: () => import('@/components/layout/AppShell.vue'), redirect: '/chat', children: [
      { path: 'chat', name: 'chat', component: () => import('@/views/ChatView.vue'), meta: { permission: 'agent_use' } },
      { path: 'image-agent', name: 'image-agent', component: () => import('@/views/ImageCanvasListView.vue'), meta: { permission: 'agent_use' } },
      { path: 'image-agent/canvases/:canvasId', name: 'image-agent-canvas', component: () => import('@/views/ImageAgentView.vue'), meta: { permission: 'agent_use' } },
      { path: 'reminders', name: 'reminders', component: () => import('@/views/RemindersView.vue'), meta: { permission: 'agent_use' } },
      { path: 'meetings', name: 'meetings', component: () => import('@/views/MeetingsView.vue'), meta: { permission: 'agent_use' } },
      { path: 'massages', name: 'massages', component: () => import('@/views/MassagesView.vue'), meta: { permission: 'agent_use' } },
      { path: 'admin/knowledge', name: 'knowledge', component: () => import('@/views/admin/KnowledgeView.vue'), meta: { permission: 'knowledge_manage' } },
      { path: 'admin/agent', name: 'agent-config', component: () => import('@/views/admin/AgentConfigView.vue'), meta: { permission: 'agent_manage' } },
      { path: 'admin/image-agent', name: 'image-agent-admin', component: () => import('@/views/admin/ImageAgentAdminView.vue'), meta: { permission: 'image_manage' } },
      { path: 'admin/notifications', name: 'notifications', component: () => import('@/views/admin/NotificationsView.vue'), meta: { permission: 'notification_manage' } },
      { path: 'admin/massage', name: 'massage-admin', component: () => import('@/views/admin/MassageAdminView.vue'), meta: { permission: 'notification_manage' } },
      { path: 'admin/work-calendar', name: 'work-calendar', component: () => import('@/views/admin/WorkCalendarView.vue'), meta: { permission: 'calendar_manage' } },
      { path: 'admin/meeting-rooms', name: 'meeting-rooms', component: () => import('@/views/admin/MeetingRoomsView.vue'), meta: { permission: 'calendar_manage' } },
      { path: 'admin/audit', name: 'audit', component: () => import('@/views/admin/AuditView.vue'), meta: { permission: 'audit_view' } },
      { path: 'admin/users', name: 'users', component: () => import('@/views/admin/UsersView.vue'), meta: { permission: 'user_manage' } },
    ],
  },
  { path: '/:pathMatch(.*)*', component: () => import('@/views/NotFoundView.vue') },
]

const router = createRouter({ history: createWebHistory(), routes })
installChunkLoadRecovery(router)
router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.initialize()
  if (!to.meta.public && !auth.user) return { name: 'login', query: { redirect: to.fullPath } }
  if (to.name === 'login' && auth.user) return { name: 'chat' }
  const permission = to.meta.permission as PermissionKey | undefined
  if (permission && !auth.can(permission)) return { name: 'chat' }
})
export default router
