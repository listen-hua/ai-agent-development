<script setup lang="ts">
import { Picture } from '@element-plus/icons-vue'
import type { NotificationDraft } from '@/types/domain'

defineProps<{ items: NotificationDraft[] }>()
const emit = defineEmits<{ approve: [id: string]; send: [id: string]; cancel: [id: string] }>()
const labels: Record<string, string> = { draft: '待审核', approved: '已审核', scheduled: '待定时发送', sending: '发送中', sent: '已发送', partial: '部分失败', failed: '发送失败', cancelled: '已取消' }
const types: Record<string, 'warning' | 'success' | 'info' | 'primary' | 'danger'> = { draft: 'warning', approved: 'primary', scheduled: 'primary', sending: 'warning', sent: 'success', partial: 'warning', failed: 'danger', cancelled: 'info' }
function recipientSummary(row: NotificationDraft) {
  if (row.recipient_type === 'legacy') return `历史范围 · ${row.recipient_count} 人`
  const unit = row.recipient_type === 'chat' ? '个群聊' : '人'
  const names = row.recipients?.slice(0, 2).map(item => item.name).join('、')
  const rest = Math.max(0, row.recipient_count - 2)
  return `${row.recipient_count} ${unit}${names ? ` · ${names}${rest ? ` 等 ${row.recipient_count} 项` : ''}` : ''}`
}
</script>

<template>
  <div class="data-table-wrap">
    <el-table :data="items" empty-text="还没有通知草稿">
      <el-table-column label="通知" min-width="280">
        <template #default="{ row }">
          <div class="notification-title">
            <div><strong>{{ row.title }}</strong><el-tag v-if="row.images?.length" size="small" type="info" effect="plain"><el-icon><Picture /></el-icon> {{ row.images.length }}</el-tag></div>
            <small>{{ row.content }}</small>
            <p v-if="row.last_error" :title="row.last_error">{{ row.last_error }}</p>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="接收目标" min-width="180">
        <template #default="{ row }"><span class="recipient-summary">{{ recipientSummary(row) }}</span></template>
      </el-table-column>
      <el-table-column label="状态" width="120">
        <template #default="{ row }"><el-tag :type="types[row.status] || 'info'" round>{{ labels[row.status] || row.status }}</el-tag></template>
      </el-table-column>
      <el-table-column label="计划时间" width="180">
        <template #default="{ row }">{{ row.scheduled_at ? new Date(row.scheduled_at).toLocaleString('zh-CN') : '审核后手动发送' }}</template>
      </el-table-column>
      <el-table-column align="right" width="230">
        <template #default="{ row }">
          <el-button v-if="row.status === 'draft'" size="small" type="primary" plain @click="emit('approve', row.id)">审核通过</el-button>
          <el-button v-if="['approved', 'scheduled', 'failed', 'partial'].includes(row.status)" size="small" type="primary" @click="emit('send', row.id)">{{ row.status === 'scheduled' ? '立即发送' : row.status === 'failed' || row.status === 'partial' ? '重试发送' : '发送到飞书' }}</el-button>
          <el-button v-if="['draft', 'approved', 'scheduled', 'failed', 'partial'].includes(row.status)" size="small" text type="danger" @click="emit('cancel', row.id)">取消</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<style scoped>
.notification-title { min-width: 0; display: flex; flex-direction: column; gap: 3px; }
.notification-title > div { display: flex; align-items: center; gap: 7px; }
.notification-title strong { overflow: hidden; color: #38445a; font-size: 14px; text-overflow: ellipsis; white-space: nowrap; }
.notification-title small { overflow: hidden; color: #9099aa; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.notification-title p { overflow: hidden; margin: 1px 0 0; color: #d55757; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.recipient-summary { color: #647087; font-size: 13px; }
</style>
