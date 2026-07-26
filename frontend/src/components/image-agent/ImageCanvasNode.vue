<script setup lang="ts">
import { computed, ref } from 'vue'
import { Loading, Picture, Warning } from '@element-plus/icons-vue'
import { imageAssetURL } from '@/services/image-agent'

const props = defineProps<{ data: { assetId?: string; status: 'pending' | 'ready' | 'failed'; error?: string } }>()
const url = computed(() => props.data.assetId ? imageAssetURL(props.data.assetId) : '')
const draggingReference = ref(false)

function dragReference(event: DragEvent) {
  if (!props.data.assetId || !event.dataTransfer) return
  event.dataTransfer.effectAllowed = 'copy'
  event.dataTransfer.setData('application/x-image-asset-id', props.data.assetId)
  event.dataTransfer.setData('text/plain', `image-asset:${props.data.assetId}`)
  draggingReference.value = true
}
</script>

<template>
  <div class="canvas-image-node" :class="[data.status, { 'dragging-reference': draggingReference }]">
    <img v-if="data.status === 'ready' && url" :src="url" alt="AI 生成图片" draggable="false">
    <div v-else-if="data.status === 'pending'" class="node-state">
      <el-icon class="is-loading"><Loading /></el-icon><strong>正在生成</strong><span>结果会在原位置出现</span>
    </div>
    <div v-else class="node-state failed">
      <el-icon><Warning /></el-icon><strong>生成失败</strong><span>{{ data.error || '请在右侧重新生成' }}</span>
    </div>
    <div
      v-if="data.status === 'ready' && data.assetId"
      class="node-reference-handle nodrag nopan"
      draggable="true"
      title="拖到右侧参考图槽位"
      @mousedown.stop
      @dragstart.stop="dragReference"
      @dragend.stop="draggingReference = false"
    >
      <el-icon><Picture /></el-icon>设为参考图
    </div>
  </div>
</template>

<style scoped>
.canvas-image-node { position: relative; width: 100%; height: 100%; overflow: hidden; border: 1px solid rgba(110, 91, 145, .22); border-radius: 13px; background: white; box-shadow: 0 12px 30px rgba(34, 27, 48, .16); }
.canvas-image-node img { width: 100%; height: 100%; display: block; object-fit: cover; }
.canvas-image-node.pending { border-style: dashed; background: linear-gradient(135deg, #faf8ff, #f3eefb); }
.canvas-image-node.failed { border-color: #efb9b9; background: #fff7f7; }
.node-state { width: 100%; height: 100%; color: #8465ac; display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 7px; text-align: center; }
.node-state .el-icon { font-size: 26px; }
.node-state strong { font-size: 14px; }.node-state span { max-width: 82%; color: #9a91a6; font-size: 11px; line-height: 1.45; }
.node-state.failed { color: #c45b5b; }
.node-reference-handle { position: absolute; z-index: 4; right: 8px; bottom: 8px; height: 28px; padding: 0 9px; border-radius: 8px; color: white; background: rgba(31, 25, 42, .82); display: flex; align-items: center; gap: 5px; font-size: 11px; cursor: grab; opacity: 0; user-select: none; transition: opacity .18s ease, background .18s ease, transform .18s ease; }
.canvas-image-node:hover .node-reference-handle, .canvas-image-node.dragging-reference .node-reference-handle { opacity: 1; }
.node-reference-handle:hover { background: rgba(104, 67, 151, .94); transform: translateY(-1px); }
.node-reference-handle:active, .canvas-image-node.dragging-reference .node-reference-handle { cursor: grabbing; }
</style>
