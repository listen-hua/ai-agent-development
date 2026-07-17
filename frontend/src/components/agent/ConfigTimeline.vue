<script setup lang="ts">
import { CircleCheck, Promotion } from '@element-plus/icons-vue'
import type { AgentConfigVersion } from '@/types/domain'
defineProps<{ versions: AgentConfigVersion[] }>(); const emit = defineEmits<{ publish: [id: string] }>(); function date(value: string) { return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }
</script>
<template><aside class="config-timeline"><h3>配置版本</h3><p>发布和回滚均保留完整审计记录。</p><div class="timeline-list"><article v-for="item in versions" :key="item.id" :class="item.status"><span class="timeline-dot"><CircleCheck v-if="item.status === 'published'" /></span><div><div><strong>版本 {{ item.version }}</strong><el-tag v-if="item.status === 'published'" type="success" size="small">线上</el-tag><el-tag v-else-if="item.status === 'draft'" type="warning" size="small">草稿</el-tag><el-tag v-else size="small">已归档</el-tag></div><small>{{ date(item.created_at) }}</small><p>{{ item.config.generation_model }} · Top {{ item.config.rerank_top_n }}</p><el-button v-if="item.status === 'draft'" size="small" type="primary" plain :icon="Promotion" @click="emit('publish', item.id)">评测并发布</el-button></div></article></div></aside></template>

