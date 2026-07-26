<script setup lang="ts">
import { onMounted } from 'vue'
import { Calendar, Refresh } from '@element-plus/icons-vue'
import PageHeader from '@/components/common/PageHeader.vue'
import MeetingRoomOverview from '@/components/meeting/MeetingRoomOverview.vue'
import MeetingRoomTable from '@/components/meeting/MeetingRoomTable.vue'
import MeetingBookingAlerts from '@/components/meeting/MeetingBookingAlerts.vue'
import { useMeetingRooms } from '@/composables/meeting/useMeetingRooms'

const page = useMeetingRooms()
onMounted(page.load)
</script>

<template>
  <section class="admin-page">
    <PageHeader eyebrow="MEETING AUTOMATION" title="会议室管理" description="同步飞书会议室、禁用状态与预定限制；只有无需审批的普通会议室会进入 AI 自动推荐。">
      <el-button v-if="!page.settings.value?.calendar_id" :icon="Calendar" :loading="page.initializing.value" @click="page.initializeCalendar">初始化共享日历</el-button>
      <el-button type="primary" :icon="Refresh" :loading="page.syncing.value" @click="page.sync">立即同步</el-button>
    </PageHeader>
    <el-alert v-if="page.settings.value?.last_sync_error" type="error" :closable="false" show-icon :title="page.settings.value.last_sync_error" />
    <MeetingRoomOverview :settings="page.settings.value" :ordinary="page.ordinaryCount.value" :restricted="page.restrictedCount.value" :alerts="page.alertCount.value" />
    <MeetingRoomTable :rooms="page.rooms.value" :loading="page.loading.value" />
    <MeetingBookingAlerts :bookings="page.bookingAlerts.value" :loading="page.loading.value" />
  </section>
</template>
