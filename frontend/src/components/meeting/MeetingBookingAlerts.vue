<script setup lang="ts">
import type { MeetingBooking } from '@/types/domain'

defineProps<{ bookings: MeetingBooking[]; loading: boolean }>()

function dateTime(value: string) {
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}
</script>

<template>
  <section class="meeting-room-table meeting-booking-alerts">
    <header>
      <div><h3>异常预约</h3><p>临时日程清理失败或改期后旧日程释放失败时会出现在这里，需要管理员到飞书日历核对处理。</p></div>
      <el-tag :type="bookings.length ? 'danger' : 'success'" effect="plain">{{ bookings.length }} 条</el-tag>
    </header>
    <el-empty v-if="!loading && !bookings.length" description="暂无需要人工处理的预约" :image-size="72" />
    <el-table v-else :data="bookings" v-loading="loading" row-key="id">
      <el-table-column prop="title" label="会议" min-width="160" />
      <el-table-column prop="room_name" label="会议室" min-width="130" />
      <el-table-column label="预约时段" min-width="180"><template #default="{ row }">{{ dateTime(row.start_at) }}</template></el-table-column>
      <el-table-column prop="event_id" label="飞书日程 ID" min-width="220" show-overflow-tooltip />
      <el-table-column prop="last_error" label="异常原因" min-width="260"><template #default="{ row }"><span class="danger-text">{{ row.last_error }}</span></template></el-table-column>
    </el-table>
  </section>
</template>
