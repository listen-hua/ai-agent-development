<script setup lang="ts">
import { ref, watch } from 'vue'
import { PictureRounded } from '@element-plus/icons-vue'
import { imagePromptActionPreviewURL } from '@/services/image-agent'
import type { ImagePromptAction } from '@/types/image-agent'

const props = defineProps<{ action: ImagePromptAction }>()
const emit = defineEmits<{ select: [action: ImagePromptAction] }>()
const loadFailed = ref(false)

watch(() => [props.action.id, props.action.updated_at], () => {
  loadFailed.value = false
})
</script>

<template>
  <el-popover
    v-if="action.has_preview"
    trigger="hover"
    placement="left-start"
    :width="300"
    :show-after="220"
    :hide-after="100"
    :persistent="false"
    popper-class="image-action-preview-popper"
  >
    <template #reference>
      <el-button plain round @click="emit('select', action)">{{ action.name }}</el-button>
    </template>
    <div class="action-preview">
      <img
        v-if="!loadFailed"
        :src="imagePromptActionPreviewURL(action.id, action.updated_at)"
        :alt="`${action.name}预览图`"
        @error="loadFailed = true"
      >
      <div v-else class="preview-error"><el-icon><PictureRounded /></el-icon><span>预览图加载失败</span></div>
    </div>
  </el-popover>
  <el-button v-else plain round @click="emit('select', action)">{{ action.name }}</el-button>
</template>

<style scoped>
.action-preview { width: 276px; height: 200px; overflow: hidden; border-radius: 10px; background: #f5f2f7; }
.action-preview img { width: 100%; height: 100%; display: block; object-fit: contain; }
.preview-error { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 8px; color: #9e96a5; font-size: 12px; }
.preview-error .el-icon { font-size: 26px; }
</style>
