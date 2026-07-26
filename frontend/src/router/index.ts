import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { installChunkLoadRecovery } from './chunkRecovery'

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { public: true } },
  {
    path: '/', component: () => import('@/components/layout/AppShell.vue'), redirect: '/chat', children: [
      { path: 'chat', name: 'chat', component: () => import('@/views/ChatView.vue') },
	  { path: 'image-agent', name: 'image-agent', component: () => import('@/views/ImageAgentView.vue') },
	  { path: 'reminders', name: 'reminders', component: () => import('@/views/RemindersView.vue') },
      { path: 'admin/knowledge', name: 'knowledge', component: () => import('@/views/admin/KnowledgeView.vue'), meta: { roles: ['knowledge_admin', 'super_admin'] } },
      { path: 'admin/agent', name: 'agent-config', component: () => import('@/views/admin/AgentConfigView.vue'), meta: { roles: ['knowledge_admin', 'super_admin'] } },
      { path: 'admin/image-agent', name: 'image-agent-admin', component: () => import('@/views/admin/ImageAgentAdminView.vue'), meta: { roles: ['image_admin', 'super_admin'] } },
      { path: 'admin/notifications', name: 'notifications', component: () => import('@/views/admin/NotificationsView.vue'), meta: { roles: ['notification_admin', 'super_admin'] } },
	  { path: 'admin/work-calendar', name: 'work-calendar', component: () => import('@/views/admin/WorkCalendarView.vue'), meta: { roles: ['notification_admin', 'super_admin'] } },
      { path: 'admin/meeting-rooms', name: 'meeting-rooms', component: () => import('@/views/admin/MeetingRoomsView.vue'), meta: { roles: ['notification_admin', 'super_admin'] } },
      { path: 'admin/audit', name: 'audit', component: () => import('@/views/admin/AuditView.vue'), meta: { roles: ['auditor', 'knowledge_admin', 'notification_admin', 'super_admin'] } },
      { path: 'admin/users', name: 'users', component: () => import('@/views/admin/UsersView.vue'), meta: { roles: ['super_admin'] } },
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
  const roles = to.meta.roles as Parameters<typeof auth.hasRole> | undefined
  if (roles && !auth.hasRole(...roles)) return { name: 'chat' }
})
export default router
