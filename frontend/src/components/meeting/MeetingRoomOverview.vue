<script setup lang="ts">
import { Calendar, CircleCheck, Warning } from '@element-plus/icons-vue'
import type { MeetingSettings } from '@/types/domain'
defineProps<{ settings?: MeetingSettings; ordinary: number; restricted: number; alerts: number }>()
</script>

<template>
  <div class="meeting-room-overview">
    <article><span class="overview-icon success"><CircleCheck /></span><div><small>可自动预约</small><strong>{{ ordinary }}</strong></div></article>
    <article><span class="overview-icon"><Calendar /></span><div><small>受限或需审批</small><strong>{{ restricted }}</strong></div></article>
    <article><span class="overview-icon warning"><Warning /></span><div><small>同步 / 预约异常</small><strong>{{ alerts }}</strong></div></article>
    <article class="calendar-status"><div><small>应用共享日历</small><strong>{{ settings?.calendar_id ? '已初始化' : '待初始化' }}</strong><p v-if="settings?.last_synced_at">上次同步 {{ new Date(settings.last_synced_at).toLocaleString('zh-CN', { hour12: false }) }}</p><p v-else>尚未同步会议室</p></div><el-tag :type="settings?.calendar_id ? 'success' : 'warning'">{{ settings?.calendar_id ? 'READY' : 'REQUIRED' }}</el-tag></article>
  </div>
</template>
