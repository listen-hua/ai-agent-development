<script setup lang="ts">
import { Calendar, Clock, Location, User } from '@element-plus/icons-vue'
import type { MeetingBooking } from '@/types/domain'

defineProps<{ bookings: MeetingBooking[]; loading: boolean; showCancel?: boolean; cancellingId?: string }>()
const emit = defineEmits<{ cancel: [booking: MeetingBooking] }>()

function formatDate(value: string) {
  return new Date(value).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', weekday: 'short', hour: '2-digit', minute: '2-digit', hour12: false })
}
</script>

<template>
  <div v-loading="loading" class="my-meeting-list">
    <el-empty v-if="!loading && !bookings.length" description="暂无会议室预约" />
    <article v-for="booking in bookings" :key="booking.id" class="meeting-item">
      <div class="meeting-date"><Calendar /><span>{{ formatDate(booking.start_at) }}</span><small>至 {{ new Date(booking.end_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }) }}</small></div>
      <div class="meeting-main">
        <header><h3>{{ booking.title }}</h3><el-tag size="small" :type="booking.status === 'active' ? 'success' : booking.status === 'needs_admin' ? 'danger' : 'info'">{{ booking.status === 'active' ? '有效' : booking.status === 'cancelled' ? '已取消' : booking.status === 'replaced' ? '已改期' : '需管理员处理' }}</el-tag></header>
        <div class="meeting-meta"><span><Location />{{ booking.room_name }}</span><span><Clock />{{ Math.round((new Date(booking.end_at).getTime() - new Date(booking.start_at).getTime()) / 60000) }} 分钟</span><span><User />{{ booking.attendees.map((item) => item.name).join('、') || '仅自己' }}</span></div>
      </div>
      <el-button v-if="showCancel && booking.status === 'active'" type="danger" plain :loading="cancellingId === booking.id" @click="emit('cancel', booking)">取消预约</el-button>
    </article>
  </div>
</template>

<style scoped>
.my-meeting-list{min-height:180px}.meeting-item{display:grid;grid-template-columns:170px minmax(0,1fr) auto;align-items:center;gap:18px;padding:18px 20px;border:1px solid #ebe8f2;border-radius:16px;background:#fff;box-shadow:0 8px 22px rgba(56,45,92,.05)}.meeting-item+.meeting-item{margin-top:12px}.meeting-date{display:grid;grid-template-columns:18px 1fr;align-items:center;gap:3px 8px;color:#6651c9}.meeting-date svg{width:18px}.meeting-date span{font-weight:700}.meeting-date small{grid-column:2;color:#918a9d}.meeting-main{min-width:0}.meeting-main header{display:flex;align-items:center;gap:8px}.meeting-main h3{margin:0;font-size:17px;color:#2e2938}.meeting-meta{display:flex;flex-wrap:wrap;gap:12px;margin-top:9px;color:#777083;font-size:13px}.meeting-meta span{display:inline-flex;align-items:center;gap:5px}.meeting-meta svg{width:15px}@media(max-width:800px){.meeting-item{grid-template-columns:1fr}.meeting-item>.el-button{justify-self:start}}
</style>
