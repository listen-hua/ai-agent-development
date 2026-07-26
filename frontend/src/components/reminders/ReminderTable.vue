<script setup lang="ts">
import { Bell, Delete, EditPen, VideoPause, VideoPlay } from '@element-plus/icons-vue'
import type { Reminder } from '@/types/domain'
import { formatReminderSchedule, reminderStatusText, reminderStatusType } from '@/utils/reminder'

defineProps<{ items: Reminder[]; loading: boolean }>()
const emit = defineEmits<{ edit: [reminder: Reminder]; action: [reminder: Reminder, action: 'pause' | 'resume' | 'delete'] }>()
</script>

<template>
  <div class="reminder-table-wrap">
    <el-table v-loading="loading" :data="items" empty-text="还没有提醒，可以在聊天中说“今天下午3点提醒我写周报”">
      <el-table-column min-width="250" label="提醒内容">
        <template #default="{ row }"><div class="reminder-title"><span><Bell /></span><div><strong>{{ row.content }}</strong><small>{{ row.source_channel === 'feishu_bot' ? '通过飞书机器人创建' : '通过网页创建' }}</small></div></div></template>
      </el-table-column>
      <el-table-column min-width="210" label="重复规则"><template #default="{ row }">{{ formatReminderSchedule(row.schedule) }}</template></el-table-column>
      <el-table-column min-width="160" label="下次提醒"><template #default="{ row }">{{ row.next_fire_at ? new Date(row.next_fire_at).toLocaleString('zh-CN', { hour12: false }) : '-' }}</template></el-table-column>
      <el-table-column width="100" label="状态"><template #default="{ row }"><el-tag size="small" :type="reminderStatusType(row.status)">{{ reminderStatusText(row.status) }}</el-tag></template></el-table-column>
      <el-table-column width="210" fixed="right" label="操作">
        <template #default="{ row }"><div class="reminder-actions"><el-button v-if="row.status === 'active' || row.status === 'paused'" link type="primary" :icon="EditPen" @click="emit('edit', row)">修改</el-button><el-button v-if="row.status === 'active'" link :icon="VideoPause" @click="emit('action', row, 'pause')">暂停</el-button><el-button v-if="row.status === 'paused'" link type="success" :icon="VideoPlay" @click="emit('action', row, 'resume')">恢复</el-button><el-button v-if="row.status !== 'cancelled'" link type="danger" :icon="Delete" @click="emit('action', row, 'delete')">删除</el-button></div></template>
      </el-table-column>
    </el-table>
  </div>
</template>

<style scoped>
.reminder-table-wrap{border:1px solid #e5e8ef;border-radius:14px;background:#fff;overflow:hidden;box-shadow:0 7px 24px rgba(30,44,72,.035)}
.reminder-title{display:flex;align-items:center;gap:11px}.reminder-title>span{width:35px;height:35px;border-radius:10px;background:#eef2ff;color:#526ed9;display:grid;place-items:center}.reminder-title>span :deep(svg){width:17px}.reminder-title>div{display:flex;min-width:0;flex-direction:column}.reminder-title strong{color:#354157;font-size: 14px}.reminder-title small{color:#9aa3b2;font-size: 11px}.reminder-actions{display:flex;align-items:center;gap:2px}.reminder-actions :deep(.el-button){margin-left:0}
</style>
