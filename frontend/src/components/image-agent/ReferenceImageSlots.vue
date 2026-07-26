<script setup lang="ts">
import { ref } from 'vue'
import { Delete, Plus, UploadFilled } from '@element-plus/icons-vue'
import { imageAssetURL } from '@/services/image-agent'
import type { ImageAsset } from '@/types/image-agent'

const props = defineProps<{ assets: ImageAsset[]; disabled?: boolean }>()
const emit = defineEmits<{
  upload: [files: File[]]
  remove: [id: string]
  'canvas-drop': [assetId: string, index?: number]
}>()
const input = ref<HTMLInputElement>()
const dragDepth = ref(0)
const draggingOver = ref(false)

function selectFiles(event: Event) {
  const files = Array.from((event.target as HTMLInputElement).files || [])
  if (files.length) emit('upload', files)
  ;(event.target as HTMLInputElement).value = ''
}

function canvasAssetId(transfer?: DataTransfer | null) {
  if (!transfer) return ''
  const custom = transfer.getData('application/x-image-asset-id').trim()
  if (custom) return custom
  const fallback = transfer.getData('text/plain').trim()
  return fallback.startsWith('image-asset:') ? fallback.slice('image-asset:'.length).trim() : ''
}

function dragEnter() {
  if (props.disabled) return
  dragDepth.value++
  draggingOver.value = true
}

function dragLeave() {
  dragDepth.value = Math.max(0, dragDepth.value - 1)
  if (dragDepth.value === 0) draggingOver.value = false
}

function dragOver(event: DragEvent) {
  if (props.disabled) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
}

function drop(event: DragEvent, index?: number) {
  dragDepth.value = 0
  draggingOver.value = false
  if (props.disabled) return
  const assetId = canvasAssetId(event.dataTransfer)
  if (assetId) {
    emit('canvas-drop', assetId, index)
    return
  }
  const files = Array.from(event.dataTransfer?.files || [])
  if (files.length) emit('upload', files)
}
</script>

<template>
  <section class="reference-section">
    <div class="section-label"><span>参考图</span><small>{{ assets.length }}/5 · 单张 10 MB / 总计 30 MB</small></div>
    <div
      class="reference-slots"
      :class="{ 'drag-active': draggingOver }"
      @dragenter.prevent="dragEnter"
      @dragleave="dragLeave"
      @dragover="dragOver"
      @drop.prevent="drop($event)"
    >
      <article v-for="(asset, assetIndex) in assets" :key="asset.id" class="reference-card" @dragover="dragOver" @drop.stop.prevent="drop($event, assetIndex)">
        <img :src="imageAssetURL(asset.id)" :alt="asset.file_name || '参考图'">
        <button type="button" title="移除参考图" @click="emit('remove', asset.id)"><el-icon><Delete /></el-icon></button>
      </article>
      <button
        v-for="index in Math.max(1, 5 - assets.length)"
        :key="`empty-${index}`"
        type="button"
        class="reference-empty"
        :disabled="disabled || assets.length >= 5"
        @dragover="dragOver"
        @drop.stop.prevent="drop($event, assets.length + index - 1)"
        @click="input?.click()"
      >
        <el-icon><Plus /></el-icon>
        <span>{{ index === 1 ? '上传或拖入' : '' }}</span>
      </button>
    </div>
    <div class="reference-tip"><el-icon><UploadFilled /></el-icon>可把画布图片的“设为参考图”手柄直接拖到这里</div>
    <input ref="input" hidden multiple type="file" accept="image/jpeg,image/png,image/webp" @change="selectFiles">
  </section>
</template>

<style scoped>
.reference-section { display: grid; gap: 8px; }
.section-label { display: flex; align-items: center; justify-content: space-between; }
.section-label span { color: #3d4657; font-size: 14px; font-weight: 650; }
.section-label small { color: #9aa2af; font-size: 11px; }
.reference-slots { min-height: 78px; padding: 3px; margin: -3px; border: 1px solid transparent; border-radius: 13px; display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 7px; transition: border-color .16s ease, background .16s ease, box-shadow .16s ease; }
.reference-slots.drag-active { border-color: #9270bd; background: #f8f3fc; box-shadow: 0 0 0 3px rgba(126, 86, 174, .1); }
.reference-card, .reference-empty { position: relative; min-width: 0; aspect-ratio: 1; overflow: hidden; border: 1px dashed #d5d9e2; border-radius: 10px; background: #fafbfc; }
.reference-card img { width: 100%; height: 100%; object-fit: cover; }
.reference-card button { position: absolute; top: 4px; right: 4px; width: 22px; height: 22px; border: 0; border-radius: 7px; color: white; background: rgba(22, 27, 36, .72); display: grid; place-items: center; cursor: pointer; }
.reference-empty { color: #9a8cab; display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 3px; cursor: pointer; }
.reference-empty:hover { border-color: #8d6dba; color: #7452a3; background: #fbf8ff; }
.reference-slots.drag-active .reference-empty { border-color: #a98acb; color: #7551a0; background: white; }
.reference-empty span { font-size: 10px; white-space: nowrap; }
.reference-tip { color: #9aa1ae; display: flex; align-items: center; gap: 5px; font-size: 11px; }
@media (max-width: 960px) { .reference-slots { grid-template-columns: repeat(5, 54px); } }
</style>
