<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { AlarmClock, Calendar, ChatDotRound, Collection, DataAnalysis, Fold, MagicStick, Message, OfficeBuilding, Picture, Setting, Sunny, SwitchButton, Tickets, User, UserFilled } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const collapsed = ref(false)
const navItems = computed(() => [
  { path: '/chat', label: '行政助手', icon: ChatDotRound, show: true },
	{ path: '/image-agent', label: 'AI 画图', icon: Picture, show: true },
	{ path: '/reminders', label: '我的提醒', icon: AlarmClock, show: true },
	{ path: '/meetings', label: '我的会议', icon: Calendar, show: true },
  { path: '/massages', label: '我的按摩', icon: Tickets, show: true },
  { path: '/admin/knowledge', label: '制度知识库', icon: Collection, show: auth.can('knowledge_manage') },
  { path: '/admin/agent', label: 'Agent 配置', icon: MagicStick, show: auth.can('agent_manage') },
  { path: '/admin/image-agent', label: '生图管理', icon: Setting, show: auth.can('image_manage') },
  { path: '/admin/notifications', label: '通知中心', icon: Message, show: auth.can('notification_manage') },
  { path: '/admin/massage', label: '按摩排号', icon: Tickets, show: auth.can('notification_manage') },
	{ path: '/admin/work-calendar', label: '工作日历', icon: Calendar, show: auth.can('calendar_manage') },
  { path: '/admin/meeting-rooms', label: '会议室管理', icon: OfficeBuilding, show: auth.can('calendar_manage') },
  { path: '/admin/audit', label: '质量与审计', icon: DataAnalysis, show: auth.can('audit_view') },
  { path: '/admin/users', label: '用户与权限', icon: UserFilled, show: auth.can('user_manage') },
].filter((item) => item.show))
const topbarCopy = computed(() => {
  if (route.path === '/chat') return '行政助手 · 制度与行政服务'
  if (route.path.startsWith('/image-agent')) return 'AI 画图 · 创意无限画布'
  if (route.path === '/reminders') return '个人服务 · 提醒与日程'
  if (route.path === '/meetings') return '个人服务 · 会议室预约'
  if (route.path === '/massages') return '个人服务 · 按摩排号'
  if (route.path === '/admin/massage') return '行政管理 · 按摩叫号'
  return 'Shimmer · AI Agent 管理控制台'
})

async function logout() { await auth.logout(); await router.push('/login') }
function isNavActive(path: string) {
  return route.path === path || (path === '/image-agent' && route.path.startsWith('/image-agent/'))
}
</script>

<template>
  <div class="app-shell" :class="{ collapsed }">
    <aside class="app-sidebar">
      <div class="brand">
        <div class="brand-mark"><Sunny /></div>
        <div v-if="!collapsed" class="brand-copy"><strong>微光</strong><span>SHIMMER · AI AGENTS</span></div>
      </div>
      <nav class="main-nav">
        <RouterLink v-for="item in navItems" :key="item.path" :to="item.path" :class="{ active: isNavActive(item.path) }" :aria-label="item.label" :title="item.label">
          <el-icon :size="19"><component :is="item.icon" /></el-icon><span v-if="!collapsed">{{ item.label }}</span>
        </RouterLink>
      </nav>
      <div class="sidebar-footer">
        <button class="collapse-button" :aria-label="collapsed ? '展开导航' : '收起导航'" :title="collapsed ? '展开导航' : '收起导航'" @click="collapsed = !collapsed"><el-icon><Fold /></el-icon><span v-if="!collapsed">收起导航</span></button>
      </div>
    </aside>
    <section class="app-stage">
      <header class="topbar">
        <div class="topbar-status"><span class="status-dot" />{{ topbarCopy }}</div>
        <el-dropdown v-if="!auth.isIAM" trigger="click">
          <button class="user-menu" :aria-label="`${auth.user?.name || '用户'}菜单`" title="用户菜单"><el-avatar :size="30" :src="auth.user?.avatar_url"><User /></el-avatar><span>{{ auth.user?.name }}</span></button>
          <template #dropdown><el-dropdown-menu><el-dropdown-item :icon="SwitchButton" @click="logout">退出登录</el-dropdown-item></el-dropdown-menu></template>
        </el-dropdown>
      </header>
      <main class="app-content"><RouterView /></main>
    </section>
  </div>
</template>
