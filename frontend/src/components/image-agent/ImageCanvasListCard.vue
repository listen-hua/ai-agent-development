<script setup lang="ts">
import { ref, watch } from 'vue'
import { Delete, Picture, Right } from '@element-plus/icons-vue'
import { imageAssetURL } from '@/services/image-agent'
import type { ImageCanvas } from '@/types/image-agent'

const props = defineProps<{ canvas: ImageCanvas; deleting?: boolean }>()
const emit = defineEmits<{ open: [canvas: ImageCanvas]; delete: [canvas: ImageCanvas] }>()
const previewFailed = ref(false)
const updated = (value: string) => new Date(value).toLocaleString('zh-CN', { hour12: false })

watch(() => props.canvas.preview_asset_id, () => { previewFailed.value = false })
</script>

<template>
  <article class="canvas-card" tabindex="0" @click="emit('open', canvas)" @keydown.enter="emit('open', canvas)">
    <div class="canvas-preview">
      <img
        v-if="canvas.preview_asset_id && !previewFailed"
        :src="imageAssetURL(canvas.preview_asset_id)"
        :alt="`${canvas.name} 预览图`"
        loading="lazy"
        @error="previewFailed = true"
      >
      <el-icon v-else><Picture /></el-icon>
      <span>{{ canvas.node_count || 0 }}</span>
    </div>
    <div class="canvas-copy">
      <h3>{{ canvas.name }}</h3>
      <p>{{ canvas.node_count || 0 }} 张图片 · 更新于 {{ updated(canvas.updated_at) }}</p>
    </div>
    <el-button class="delete-button" text circle :icon="Delete" :loading="deleting" aria-label="删除画布" @click.stop="emit('delete', canvas)" />
    <el-icon class="open-icon"><Right /></el-icon>
  </article>
</template>

<style scoped>
.canvas-card{position:relative;display:grid;grid-template-columns:76px minmax(0,1fr) 32px;align-items:center;gap:14px;min-height:106px;padding:15px;border:1px solid #e3deea;border-radius:18px;background:#fff;box-shadow:0 9px 28px rgba(48,35,66,.06);cursor:pointer;transition:.2s}.canvas-card:hover,.canvas-card:focus-visible{border-color:#a98ac7;transform:translateY(-2px);box-shadow:0 15px 34px rgba(75,51,102,.13);outline:none}.canvas-preview{position:relative;display:grid;place-items:center;width:76px;height:76px;overflow:hidden;border-radius:14px;color:#8a62b0;background:linear-gradient(145deg,#f2edf7,#e8dff2);font-size:27px}.canvas-preview img{width:100%;height:100%;object-fit:cover}.canvas-preview span{position:absolute;right:6px;bottom:5px;min-width:20px;padding:2px 5px;border-radius:9px;color:#fff;background:rgba(70,45,94,.82);font-size:10px;text-align:center;box-shadow:0 2px 8px rgba(36,24,48,.18)}.canvas-copy{min-width:0}.canvas-copy h3{overflow:hidden;margin:0 0 8px;color:#352d40;font-size:16px;text-overflow:ellipsis;white-space:nowrap}.canvas-copy p{margin:0;color:#958d9c;font-size:11px}.delete-button{position:absolute;top:8px;right:8px;opacity:0;color:#a66a79}.canvas-card:hover .delete-button,.delete-button:focus{opacity:1}.open-icon{color:#9d8baa;font-size:17px}
</style>
