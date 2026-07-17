<script setup lang="ts">
import { Connection, Loading, Plus, Refresh } from '@element-plus/icons-vue'
import type { KnowledgeSource } from '@/types/domain'

defineProps<{ sources: KnowledgeSource[] }>()
const emit = defineEmits<{ add: []; sync: [source: KnowledgeSource] }>()

const statusMap: Record<string, string> = {
  idle: '等待首次同步',
  queued: '等待同步',
  syncing: '正在同步',
  success: '同步成功',
  partial: '部分文件同步失败',
  error: '同步失败',
}

function isRunning(source: KnowledgeSource) {
  return source.sync_status === 'queued' || source.sync_status === 'syncing'
}

function summary(source: KnowledgeSource) {
  const stats = source.last_sync_stats
  if (!stats || source.sync_status === 'idle') return statusMap[source.sync_status] || source.sync_status
  const changed = stats.created + stats.updated
  const details = changed ? `，新增/更新 ${changed}` : ''
  return `${statusMap[source.sync_status] || source.sync_status}${details}`
}

function sourceTypeLabel(source: KnowledgeSource) {
  return source.type === 'feishu_wiki' ? '知识库' : '云空间文件夹'
}
</script>

<template>
  <div class="source-strip">
    <div class="section-label">
      <span>资料源</span>
      <el-button text :icon="Plus" @click="emit('add')">新增</el-button>
    </div>
    <button
      v-for="source in sources"
      :key="source.id"
      :disabled="isRunning(source)"
      :title="source.sync_error || '点击立即同步'"
      @click="emit('sync', source)"
    >
      <span class="source-icon"><Connection /></span>
      <span>
        <strong>{{ source.name }}</strong>
        <small>{{ sourceTypeLabel(source) }}</small>
        <small :class="{ 'source-error': source.sync_status === 'error' || source.sync_status === 'partial' }">
          {{ summary(source) }}
        </small>
      </span>
      <el-icon :class="{ 'is-loading': isRunning(source) }">
        <Loading v-if="isRunning(source)" />
        <Refresh v-else />
      </el-icon>
    </button>
  </div>
</template>

<style scoped>
button:disabled { cursor: wait; opacity: 0.78; }
.source-error { color: #d97706 !important; }
</style>
