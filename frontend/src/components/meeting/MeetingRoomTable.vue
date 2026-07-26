<script setup lang="ts">
import type { MeetingRoom } from '@/types/domain'
defineProps<{ rooms: MeetingRoom[]; loading: boolean }>()
function secondsText(value: number) { const hour = Math.floor(value / 3600); const minute = Math.floor((value % 3600) / 60); return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}` }
function roomStatus(room: MeetingRoom) {
  if (room.last_error) return { label: '同步异常', type: 'danger' as const }
  if (!room.enabled || !room.schedule_enabled) return { label: '已禁用', type: 'info' as const }
  if (room.approval_switch === 1 && room.approval_condition === 0) return { label: '全部需审批', type: 'warning' as const }
  if (room.approval_switch === 1) return { label: `超 ${room.approval_duration_hours}h 需审批`, type: 'warning' as const }
  return { label: '可自动预约', type: 'success' as const }
}
</script>

<template>
  <section class="meeting-room-table panel-card">
    <header><div><h3>飞书会议室</h3><p>AI 只会推荐启用、时段合规且本次预约不触发审批的会议室。</p></div><el-tag effect="plain">{{ rooms.length }} 间</el-tag></header>
    <el-table :data="rooms" v-loading="loading" row-key="room_id">
      <el-table-column label="会议室" min-width="210"><template #default="{ row }"><div class="room-name"><strong>{{ row.name }}</strong><small>{{ row.path?.join(' / ') || row.room_level_id || '未配置楼层' }}</small></div></template></el-table-column>
      <el-table-column prop="capacity" label="容量" width="90"><template #default="{ row }">{{ row.capacity }} 人</template></el-table-column>
      <el-table-column label="可预定时段" width="150"><template #default="{ row }">{{ secondsText(row.reservation_start_seconds) }}–{{ secondsText(row.reservation_end_seconds) }}</template></el-table-column>
      <el-table-column label="最长时长" width="100"><template #default="{ row }">{{ row.max_duration_hours }} 小时</template></el-table-column>
      <el-table-column label="自动预约状态" min-width="160"><template #default="{ row }"><el-tag :type="roomStatus(row).type" effect="light">{{ roomStatus(row).label }}</el-tag><small v-if="row.disable_reason" class="room-reason">{{ row.disable_reason }}</small></template></el-table-column>
      <el-table-column label="异常" min-width="220"><template #default="{ row }"><span :class="{ 'danger-text': row.last_error }">{{ row.last_error || '—' }}</span></template></el-table-column>
    </el-table>
  </section>
</template>
