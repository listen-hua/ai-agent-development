<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Delete, UploadFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { imagePromptActionPreviewURL } from '@/services/image-agent'
import { validatePromptPreviewFile } from '@/utils/imagePromptPreview'
import type { ImagePromptAction, ImagePromptActionPreviewChange } from '@/types/image-agent'

const props = defineProps<{ action?: ImagePromptAction }>()
const model = defineModel<ImagePromptActionPreviewChange>({ required: true })
const input = ref<HTMLInputElement>()
const localURL = ref('')
const loadFailed = ref(false)

const existingURL = computed(() => {
  if (!props.action?.has_preview || model.value.remove) return ''
  return imagePromptActionPreviewURL(props.action.id, props.action.updated_at)
})
const previewURL = computed(() => localURL.value || existingURL.value)

function choose() {
  input.value?.click()
}

function select(event: Event) {
  const element = event.target as HTMLInputElement
  const file = element.files?.[0]
  element.value = ''
  if (!file) return
  const error = validatePromptPreviewFile(file)
  if (error) {
    ElMessage.error(error)
    return
  }
  revokeLocalURL()
  localURL.value = URL.createObjectURL(file)
  loadFailed.value = false
  model.value = { file, remove: false }
}

function remove() {
  revokeLocalURL()
  loadFailed.value = false
  model.value = { remove: Boolean(props.action?.has_preview) }
}

function revokeLocalURL() {
  if (!localURL.value) return
  URL.revokeObjectURL(localURL.value)
  localURL.value = ''
}

watch(() => props.action?.id, () => {
  revokeLocalURL()
  loadFailed.value = false
})
onBeforeUnmount(revokeLocalURL)
</script>

<template>
  <div class="preview-picker">
    <div class="preview-frame" :class="{ empty: !previewURL }">
      <img v-if="previewURL && !loadFailed" :src="previewURL" alt="功能按键预览图" @error="loadFailed = true">
      <div v-else class="preview-empty">
        <el-icon><UploadFilled /></el-icon>
        <span>{{ loadFailed ? '预览图加载失败' : '尚未设置预览图' }}</span>
      </div>
    </div>
    <div class="preview-actions">
      <input ref="input" type="file" accept="image/jpeg,image/png,image/webp" hidden @change="select">
      <el-button :icon="UploadFilled" @click="choose">{{ previewURL ? '替换图片' : '选择图片' }}</el-button>
      <el-button v-if="previewURL" type="danger" plain :icon="Delete" @click="remove">移除</el-button>
      <small>选填 · JPG、PNG、WEBP · 不超过 5 MB</small>
    </div>
  </div>
</template>

<style scoped>
.preview-picker { display: grid; grid-template-columns: 190px 1fr; gap: 14px; align-items: center; }
.preview-frame { width: 190px; height: 124px; overflow: hidden; border: 1px solid #ddd6e7; border-radius: 12px; background: #f6f3f8; }
.preview-frame img { width: 100%; height: 100%; display: block; object-fit: contain; }
.preview-empty { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 7px; color: #a59dac; font-size: 12px; }
.preview-empty .el-icon { font-size: 24px; }
.preview-actions { display: flex; align-items: flex-start; flex-wrap: wrap; gap: 8px; }
.preview-actions .el-button { margin: 0; }
.preview-actions small { width: 100%; color: #9991a2; line-height: 1.5; }
@media (max-width: 640px) { .preview-picker { grid-template-columns: 1fr; }.preview-frame { width: 100%; height: 150px; } }
</style>
