<script setup lang="ts">
import { RefreshRight } from '@element-plus/icons-vue'
import type { ImageCanvas } from '@/types/image-agent'

defineProps<{ modelValue: boolean; items: ImageCanvas[]; actingId?: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; restore: [canvas: ImageCanvas] }>()
function remaining(value?: string) { if (!value) return 30; return Math.max(0, Math.ceil((new Date(value).getTime() + 30*86400000 - Date.now()) / 86400000)) }
</script>

<template>
  <el-drawer :model-value="modelValue" title="画布回收站" size="420px" @update:model-value="emit('update:modelValue',$event)">
    <el-empty v-if="!items.length" description="回收站为空" />
    <div v-else class="trash-list"><article v-for="canvas in items" :key="canvas.id"><div><strong>{{ canvas.name }}</strong><span>{{ canvas.node_count || 0 }} 张图片 · 剩余 {{ remaining(canvas.deleted_at) }} 天</span></div><el-button :icon="RefreshRight" :loading="actingId===canvas.id" @click="emit('restore',canvas)">恢复</el-button></article></div>
  </el-drawer>
</template>

<style scoped>
.trash-list{display:grid;gap:10px}.trash-list article{display:flex;align-items:center;gap:12px;padding:13px;border:1px solid #ebe6ef;border-radius:13px}.trash-list article>div{display:flex;min-width:0;flex:1;flex-direction:column;gap:5px}.trash-list strong{overflow:hidden;color:#403647;text-overflow:ellipsis;white-space:nowrap}.trash-list span{color:#998f9f;font-size:11px}
</style>
