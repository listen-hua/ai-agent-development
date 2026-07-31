<script setup lang="ts">
import { computed, ref } from 'vue'
import { Delete, Loading, Picture, Warning } from '@element-plus/icons-vue'
import { imageAssetURL } from '@/services/image-agent'

const props = defineProps<{
  data: { assetId?: string; status: 'pending' | 'ready' | 'failed'; error?: string }
  readonly?: boolean
  deleting?: boolean
  deleteDisabled?: boolean
}>()
const emit = defineEmits<{ delete: [] }>()
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
    <button
      v-if="!readonly"
      class="node-delete-button nodrag nopan"
      type="button"
      title="从画布删除"
      :disabled="deleteDisabled"
      @mousedown.stop
      @click.stop="emit('delete')"
    >
      <el-icon :class="{ 'is-loading': deleting }"><Loading v-if="deleting" /><Delete v-else /></el-icon>
    </button>
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
.node-delete-button { position: absolute; z-index: 6; top: 8px; right: 8px; width: 30px; height: 30px; padding: 0; border: 1px solid rgba(154, 66, 76, .2); border-radius: 9px; color: #a74551; background: rgba(255, 255, 255, .92); box-shadow: 0 5px 16px rgba(57, 35, 41, .13); display: grid; place-items: center; cursor: pointer; opacity: 0; transform: translateY(-2px); transition: opacity .18s ease, transform .18s ease, color .18s ease, background .18s ease; }
.canvas-image-node:hover .node-delete-button, .node-delete-button:focus-visible, .node-delete-button:disabled { opacity: 1; transform: translateY(0); }
.node-delete-button:hover:not(:disabled) { color: white; background: #bd5360; }
.node-delete-button:disabled { cursor: wait; }
.node-state { width: 100%; height: 100%; color: #8465ac; display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 7px; text-align: center; }
.node-state .el-icon { font-size: 26px; }
.node-state strong { font-size: 14px; }.node-state span { max-width: 82%; color: #9a91a6; font-size: 11px; line-height: 1.45; }
.node-state.failed { color: #c45b5b; }
.node-reference-handle { position: absolute; z-index: 4; right: 8px; bottom: 8px; height: 28px; padding: 0 9px; border-radius: 8px; color: white; background: rgba(31, 25, 42, .82); display: flex; align-items: center; gap: 5px; font-size: 11px; cursor: grab; opacity: 0; user-select: none; transition: opacity .18s ease, background .18s ease, transform .18s ease; }
.canvas-image-node:hover .node-reference-handle, .canvas-image-node.dragging-reference .node-reference-handle { opacity: 1; }
.node-reference-handle:hover { background: rgba(104, 67, 151, .94); transform: translateY(-1px); }
.node-reference-handle:active, .canvas-image-node.dragging-reference .node-reference-handle { cursor: grabbing; }
</style>
