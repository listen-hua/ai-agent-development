<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Delete, Loading, Picture, Warning } from '@element-plus/icons-vue'
import { imageAssetURL } from '@/services/image-agent'
import ImageGenerationSourceBadge from './ImageGenerationSourceBadge.vue'

const props = defineProps<{
	data: { assetId?: string; status: 'pending' | 'ready' | 'failed'; error?: string; backgroundRemoval?: boolean; requestedSize?: string; actualWidth?: number; actualHeight?: number; resolutionWarning?: string; generationRelayName?: string; generationModelName?: string; generationModelKey?: string }
  readonly?: boolean
  deleting?: boolean
  deleteDisabled?: boolean
}>()
const emit = defineEmits<{ delete: [] }>()
const url = computed(() => props.data.assetId ? imageAssetURL(props.data.assetId) : '')
const draggingReference = ref(false)
const naturalSize = ref({ width: 0, height: 0 })
const resolution = computed(() => naturalSize.value.width > 0 && naturalSize.value.height > 0
  ? `${naturalSize.value.width} × ${naturalSize.value.height}`
	: '')
const requestedResolution = computed(() => props.data.requestedSize || '')

watch(() => props.data.assetId, () => {
  naturalSize.value = { width: 0, height: 0 }
})

function captureResolution(event: Event) {
  const image = event.currentTarget as HTMLImageElement
  naturalSize.value = { width: image.naturalWidth, height: image.naturalHeight }
}

function dragReference(event: DragEvent) {
  if (!props.data.assetId || !event.dataTransfer) return
  event.dataTransfer.effectAllowed = 'copy'
  event.dataTransfer.setData('application/x-image-asset-id', props.data.assetId)
  event.dataTransfer.setData('text/plain', `image-asset:${props.data.assetId}`)
  draggingReference.value = true
}
</script>

<template>
  <div class="canvas-image-node" :class="[data.status, { 'dragging-reference': draggingReference, 'transparent-result': data.backgroundRemoval }]" :tabindex="data.status === 'ready' ? 0 : -1">
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
    <ImageGenerationSourceBadge
      v-if="data.status === 'ready' && (data.generationRelayName || data.generationModelName || data.generationModelKey || data.backgroundRemoval)"
      :relay-name="data.generationRelayName"
      :model-name="data.generationModelName"
      :model-key="data.generationModelKey"
      :pixian="data.backgroundRemoval"
    />
    <img v-if="data.status === 'ready' && url" :src="url" alt="AI 生成图片" draggable="false" @load="captureResolution">
	<div v-if="data.status === 'ready' && resolution" class="node-resolution" :class="{ warning: data.resolutionWarning }" :title="data.resolutionWarning || `图片实际分辨率：${resolution}`">
	  <span v-if="requestedResolution">请求：{{ requestedResolution }}</span><span>实际：{{ resolution }}</span>
	  <small v-if="data.resolutionWarning"><el-icon><Warning /></el-icon>分辨率已降级</small>
	</div>
    <div v-else-if="data.status === 'pending'" class="node-state">
      <el-icon class="is-loading"><Loading /></el-icon><strong>{{ data.backgroundRemoval ? '正在智能抠图' : '正在生成' }}</strong><span>{{ data.backgroundRemoval ? '透明 PNG 会出现在原图右侧' : '结果会在原位置出现' }}</span>
    </div>
    <div v-else class="node-state failed">
      <el-icon><Warning /></el-icon><strong>{{ data.backgroundRemoval ? '抠图失败' : '生成失败' }}</strong><span>{{ data.error || '请稍后重试' }}</span>
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
.canvas-image-node.transparent-result { background-color: #fff; background-image: linear-gradient(45deg, #e8e8eb 25%, transparent 25%), linear-gradient(-45deg, #e8e8eb 25%, transparent 25%), linear-gradient(45deg, transparent 75%, #e8e8eb 75%), linear-gradient(-45deg, transparent 75%, #e8e8eb 75%); background-size: 24px 24px; background-position: 0 0, 0 12px, 12px -12px, -12px 0; }
.canvas-image-node.transparent-result img { object-fit: contain; }
.canvas-image-node.pending { border-style: dashed; background: linear-gradient(135deg, #faf8ff, #f3eefb); }
.canvas-image-node.failed { border-color: #efb9b9; background: #fff7f7; }
.node-delete-button { position: absolute; z-index: 6; top: 8px; right: 8px; width: 30px; height: 30px; padding: 0; border: 1px solid rgba(154, 66, 76, .2); border-radius: 9px; color: #a74551; background: rgba(255, 255, 255, .92); box-shadow: 0 5px 16px rgba(57, 35, 41, .13); display: grid; place-items: center; cursor: pointer; opacity: 0; transform: translateY(-2px); transition: opacity .18s ease, transform .18s ease, color .18s ease, background .18s ease; }
.canvas-image-node:hover .node-delete-button, .node-delete-button:focus-visible, .node-delete-button:disabled { opacity: 1; transform: translateY(0); }
.canvas-image-node:hover :deep(.generation-source-badge), .canvas-image-node:focus-visible :deep(.generation-source-badge) { opacity: 1; transform: translateY(0); }
.node-delete-button:hover:not(:disabled) { color: white; background: #bd5360; }
.node-delete-button:disabled { cursor: wait; }
.node-state { width: 100%; height: 100%; color: #8465ac; display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 7px; text-align: center; }
.node-state .el-icon { font-size: 26px; }
.node-state strong { font-size: 14px; }.node-state span { max-width: 82%; color: #9a91a6; font-size: 11px; line-height: 1.45; }
.node-state.failed { color: #c45b5b; }
.node-resolution { position: absolute; z-index: 4; left: 8px; bottom: 8px; padding: 6px 8px; border: 1px solid rgba(255, 255, 255, .2); border-radius: 7px; color: white; background: rgba(31, 25, 42, .82); box-shadow: 0 4px 14px rgba(26, 20, 35, .18); display:flex; flex-direction:column; gap:3px; font-size: 11px; font-variant-numeric: tabular-nums; line-height: 1.15; pointer-events: none; backdrop-filter: blur(5px); opacity: 0; transform: translateY(2px); transition: opacity .18s ease, transform .18s ease; }
.node-resolution.warning { border-color:rgba(255,190,92,.55); }.node-resolution small { color:#ffd184; display:flex; align-items:center; gap:3px; font-size:10px; }
.canvas-image-node:hover .node-resolution, .canvas-image-node:focus-visible .node-resolution { opacity: 1; transform: translateY(0); }
.node-reference-handle { position: absolute; z-index: 4; right: 8px; bottom: 8px; height: 28px; padding: 0 9px; border-radius: 8px; color: white; background: rgba(31, 25, 42, .82); display: flex; align-items: center; gap: 5px; font-size: 11px; cursor: grab; opacity: 0; user-select: none; transition: opacity .18s ease, background .18s ease, transform .18s ease; }
.canvas-image-node:hover .node-reference-handle, .canvas-image-node.dragging-reference .node-reference-handle { opacity: 1; }
.node-reference-handle:hover { background: rgba(104, 67, 151, .94); transform: translateY(-1px); }
.node-reference-handle:active, .canvas-image-node.dragging-reference .node-reference-handle { cursor: grabbing; }
</style>
