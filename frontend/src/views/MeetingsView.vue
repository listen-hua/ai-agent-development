<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Calendar, Refresh } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import MyMeetingList from '@/components/meeting/MyMeetingList.vue'
import { meetingService } from '@/services/meeting'
import type { MeetingBooking } from '@/types/domain'

const bookings = ref<MeetingBooking[]>([])
const loading = ref(false)
const cancellingId = ref('')
const tab = ref<'upcoming' | 'history'>('upcoming')
const now = () => Date.now()
const upcoming = computed(() => bookings.value.filter((item) => item.status === 'active' && new Date(item.end_at).getTime() > now()).sort((a, b) => +new Date(a.start_at) - +new Date(b.start_at)))
const history = computed(() => bookings.value.filter((item) => item.status !== 'active' || new Date(item.end_at).getTime() <= now()))

async function load() {
  loading.value = true
  try { bookings.value = await meetingService.listBookings() } finally { loading.value = false }
}

async function cancelBooking(booking: MeetingBooking) {
  let actionId = ''
  try {
    await ElMessageBox.confirm(`确认取消“${booking.title}”吗？取消后会释放 ${booking.room_name}，并通知所有参会人。`, '取消会议室预约', { type: 'warning', confirmButtonText: '继续', cancelButtonText: '暂不取消' })
    cancellingId.value = booking.id
    const action = await meetingService.prepareCancellation(booking.id)
    actionId = action.id
    await ElMessageBox.confirm('这是最后一步。确认后将删除飞书日程，且无法在微光中撤销。', '确认取消', { type: 'warning', confirmButtonText: '确认取消预约', cancelButtonText: '返回' })
    await meetingService.confirmAction(action.id)
    ElMessage.success('会议室预约已取消')
    await load()
  } catch (error) {
    if ((error === 'cancel' || error === 'close') && actionId) await meetingService.cancelAction(actionId).catch(() => undefined)
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error instanceof Error ? error.message : '取消预约失败')
  } finally { cancellingId.value = '' }
}

onMounted(load)
</script>

<template>
  <section class="meetings-page">
    <header class="page-hero"><div class="hero-icon"><Calendar /></div><div><p>PERSONAL MEETINGS</p><h1>我的会议</h1><span>查看和取消由行政助手创建的会议室预约</span></div><el-button :icon="Refresh" :loading="loading" @click="load">刷新</el-button></header>
    <el-tabs v-model="tab" class="meeting-tabs">
      <el-tab-pane :label="`即将开始 ${upcoming.length}`" name="upcoming"><MyMeetingList :bookings="upcoming" :loading="loading" show-cancel :cancelling-id="cancellingId" @cancel="cancelBooking" /></el-tab-pane>
      <el-tab-pane :label="`历史记录 ${history.length}`" name="history"><MyMeetingList :bookings="history" :loading="loading" /></el-tab-pane>
    </el-tabs>
  </section>
</template>

<style scoped>
.meetings-page{max-width:1160px;margin:0 auto;padding:28px}.page-hero{display:flex;align-items:center;gap:15px;margin-bottom:20px;padding:22px 24px;border-radius:20px;color:#fff;background:linear-gradient(125deg,#4d3c9f,#7d5ee8 65%,#9f86ff);box-shadow:0 18px 42px rgba(88,65,176,.24)}.hero-icon{display:grid;place-items:center;width:52px;height:52px;border-radius:16px;background:rgba(255,255,255,.16)}.hero-icon svg{width:26px}.page-hero>div:nth-child(2){flex:1}.page-hero p{margin:0 0 2px;font-size:11px;letter-spacing:.16em;opacity:.72}.page-hero h1{margin:0 0 3px;font-size:25px}.page-hero span{font-size:13px;opacity:.82}.meeting-tabs{padding:8px 20px 22px;border:1px solid #ebe8f2;border-radius:20px;background:#fbfaff}@media(max-width:700px){.meetings-page{padding:16px}.page-hero{align-items:flex-start;flex-wrap:wrap}.page-hero>.el-button{margin-left:67px}}
</style>
