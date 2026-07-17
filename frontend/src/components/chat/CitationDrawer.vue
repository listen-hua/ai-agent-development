<script setup lang="ts">
import { computed } from 'vue'
import { Link } from '@element-plus/icons-vue'
import type { Citation } from '@/types/domain'
const props = defineProps<{ modelValue: boolean; citation: Citation | null }>(); const emit = defineEmits<{ 'update:modelValue': [value: boolean] }>(); const title = computed(() => props.citation?.title || '制度引用')
</script>
<template><el-drawer :model-value="modelValue" :title="title" size="min(460px, 92vw)" @update:model-value="emit('update:modelValue', $event)"><div v-if="citation" class="citation-detail"><div class="citation-meta"><span>版本 {{ citation.version || '当前版' }}</span><span v-if="citation.page">第 {{ citation.page }} 页</span><span v-if="citation.heading">{{ citation.heading }}</span></div><blockquote>{{ citation.excerpt }}</blockquote><a v-if="citation.source_url" :href="citation.source_url" target="_blank"><el-icon><Link /></el-icon>在飞书中打开原文</a><div class="citation-security">该引用来自你有权访问的已发布制度版本。</div></div></el-drawer></template>

