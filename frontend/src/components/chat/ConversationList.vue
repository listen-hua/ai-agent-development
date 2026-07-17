<script setup lang="ts">
import { Delete, EditPen, Plus } from '@element-plus/icons-vue'
import type { Conversation } from '@/types/domain'
defineProps<{ conversations: Conversation[]; activeId: string }>()
const emit = defineEmits<{ select: [id: string]; create: []; remove: [id: string] }>()
function relative(value: string) { const days = Math.floor((Date.now() - new Date(value).getTime()) / 86400000); return days <= 0 ? '今天' : days === 1 ? '昨天' : `${days} 天前` }
</script>
<template>
  <aside class="conversation-panel">
    <div class="conversation-heading"><div><span>对话空间</span><strong>制度咨询</strong></div><el-button circle :icon="Plus" @click="emit('create')" /></div>
    <div v-if="conversations.length" class="conversation-list">
      <button v-for="item in conversations" :key="item.id" :class="{ active: item.id === activeId }" @click="emit('select', item.id)">
        <el-icon><EditPen /></el-icon><span><strong>{{ item.title }}</strong><small>{{ relative(item.updated_at) }}</small></span><el-icon class="delete-conversation" @click.stop="emit('remove', item.id)"><Delete /></el-icon>
      </button>
    </div>
    <div v-else class="conversation-empty"><span>还没有历史对话</span><small>开始提问后会保存在这里</small></div>
    <div class="conversation-policy"><span>90 天</span><p>问答记录默认留存周期</p></div>
  </aside>
</template>

