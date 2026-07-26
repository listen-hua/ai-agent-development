<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { AlarmClock, Calendar, ChatDotRound, Collection, DataAnalysis, Fold, MagicStick, Message, OfficeBuilding, Operation, Picture, Setting, SwitchButton, User, UserFilled } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const collapsed = ref(false)
const navItems = computed(() => [
  { path: '/chat', label: '行政助手', icon: ChatDotRound, show: true },
	{ path: '/image-agent', label: 'AI 画图', icon: Picture, show: true },
	{ path: '/reminders', label: '我的提醒', icon: AlarmClock, show: true },
  { path: '/admin/knowledge', label: '制度知识库', icon: Collection, show: auth.hasRole('knowledge_admin') },
  { path: '/admin/agent', label: 'Agent 配置', icon: MagicStick, show: auth.hasRole('knowledge_admin') },
  { path: '/admin/image-agent', label: '生图管理', icon: Setting, show: auth.hasRole('image_admin') },
  { path: '/admin/notifications', label: '通知中心', icon: Message, show: auth.hasRole('notification_admin') },
	{ path: '/admin/work-calendar', label: '工作日历', icon: Calendar, show: auth.hasRole('notification_admin') },
  { path: '/admin/meeting-rooms', label: '会议室管理', icon: OfficeBuilding, show: auth.hasRole('notification_admin') },
  { path: '/admin/audit', label: '质量与审计', icon: DataAnalysis, show: auth.hasRole('auditor', 'knowledge_admin', 'notification_admin') },
  { path: '/admin/users', label: '用户与权限', icon: UserFilled, show: auth.hasRole('super_admin') },
].filter((item) => item.show))
const topbarCopy = computed(() => route.path === '/image-agent' ? '企业内部 · AI 创意画布' : '企业内部 · 严格依据制度回答')

async function logout() { await auth.logout(); await router.push('/login') }
</script>

<template>
  <div class="app-shell" :class="{ collapsed }">
    <aside class="app-sidebar">
      <div class="brand">
        <div class="brand-mark"><Operation /></div>
        <div v-if="!collapsed" class="brand-copy"><strong>知行</strong><span>行政 AI 助手</span></div>
      </div>
      <nav class="main-nav">
        <RouterLink v-for="item in navItems" :key="item.path" :to="item.path" :class="{ active: route.path === item.path }" :aria-label="item.label" :title="item.label">
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
        <el-dropdown trigger="click">
          <button class="user-menu" :aria-label="`${auth.user?.name || '用户'}菜单`" title="用户菜单"><el-avatar :size="30" :src="auth.user?.avatar_url"><User /></el-avatar><span>{{ auth.user?.name }}</span></button>
          <template #dropdown><el-dropdown-menu><el-dropdown-item :icon="SwitchButton" @click="logout">退出登录</el-dropdown-item></el-dropdown-menu></template>
        </el-dropdown>
      </header>
      <main class="app-content"><RouterView /></main>
    </section>
  </div>
</template>
