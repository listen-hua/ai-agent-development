<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { Delete, Picture, MagicStick } from '@element-plus/icons-vue'
import { ElInput } from 'element-plus'
import type { NotificationImage } from '@/types/domain'

const props = defineProps<{ modelValue: string; images: NotificationImage[]; uploading: boolean; drafting: boolean }>()
const emit = defineEmits<{
  'update:modelValue': [value: string]
  upload: [file: File]
  removeImage: [index: number]
  aiDraft: [brief: string]
}>()
const input = ref<InstanceType<typeof ElInput>>()
const fileInput = ref<HTMLInputElement>()

function insert(before: string, after = '', placeholder = '文字') {
  const textarea = input.value?.textarea
  if (!textarea) return
  const start = textarea.selectionStart
  const end = textarea.selectionEnd
  const selected = props.modelValue.slice(start, end) || placeholder
  const value = `${props.modelValue.slice(0, start)}${before}${selected}${after}${props.modelValue.slice(end)}`
  emit('update:modelValue', value)
  nextTick(() => {
    textarea.focus()
    textarea.setSelectionRange(start + before.length, start + before.length + selected.length)
  })
}

function chooseImage(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) emit('upload', file)
  target.value = ''
}
</script>

<template>
  <div class="rich-editor">
    <div class="rich-toolbar" aria-label="正文格式工具栏">
      <el-button text title="一级标题" @click="insert('# ', '', '标题')">H1</el-button>
      <el-button text title="二级标题" @click="insert('## ', '', '小标题')">H2</el-button>
      <el-button text title="加粗" @click="insert('**', '**')"><strong>B</strong></el-button>
      <el-button text title="项目列表" @click="insert('- ', '', '列表项')">• 列表</el-button>
      <el-button text title="链接" @click="insert('[', '](https://)', '链接文字')">链接</el-button>
      <span />
      <input ref="fileInput" hidden type="file" accept="image/jpeg,image/png,image/gif,image/webp,image/bmp,image/tiff,image/x-icon" @change="chooseImage">
      <el-button text type="primary" :icon="Picture" :loading="uploading" :disabled="images.length >= 9" @click="fileInput?.click()">添加图片</el-button>
    </div>
    <el-input
      ref="input"
      :model-value="modelValue"
      type="textarea"
      :rows="11"
      maxlength="4000"
      show-word-limit
      resize="vertical"
      placeholder="输入通知正文。支持标题、加粗、列表和链接等 Markdown 富文本格式。"
      @update:model-value="emit('update:modelValue', $event)"
    />
    <div v-if="images.length" class="image-strip">
      <article v-for="(image, index) in images" :key="image.image_key">
        <img v-if="image.preview_url" :src="image.preview_url" :alt="image.alt">
        <div v-else class="image-placeholder"><el-icon><Picture /></el-icon></div>
        <span :title="image.name">{{ image.name }}</span>
        <el-button circle text type="danger" :icon="Delete" aria-label="删除图片" @click="emit('removeImage', index)" />
      </article>
    </div>
    <div class="rich-editor-actions">
      <small>最多 9 张图片，单张不超过 10 MB。图片会先上传到飞书，再随卡片发送。</small>
      <el-button text type="primary" :icon="MagicStick" :loading="drafting" :disabled="!modelValue.trim()" @click="emit('aiDraft', modelValue)">AI 润色正文</el-button>
    </div>
  </div>
</template>

<style scoped>
.rich-editor { width: 100%; border: 1px solid #dfe4ec; border-radius: 10px; overflow: hidden; background: #fff; }
.rich-toolbar { min-height: 40px; display: flex; align-items: center; gap: 1px; padding: 4px 7px; border-bottom: 1px solid #e8ebf1; background: #f8f9fb; }
.rich-toolbar > span { flex: 1; }
.rich-toolbar :deep(.el-button) { margin: 0; padding: 7px 8px; color: #566176; }
.rich-editor :deep(.el-textarea__inner) { border: 0; border-radius: 0; box-shadow: none; padding: 14px; line-height: 1.7; }
.image-strip { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; padding: 10px 12px; border-top: 1px solid #edf0f4; }
.image-strip article { min-width: 0; display: grid; grid-template-columns: 38px 1fr 27px; gap: 7px; align-items: center; padding: 6px; border: 1px solid #e5e9f0; border-radius: 8px; }
.image-strip img, .image-placeholder { width: 38px; height: 38px; border-radius: 6px; object-fit: cover; background: #eef2f7; display: grid; place-items: center; color: #7d8ba4; }
.image-strip span { overflow: hidden; color: #59657a; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.rich-editor-actions { display: flex; justify-content: space-between; align-items: center; gap: 12px; padding: 7px 12px; border-top: 1px solid #edf0f4; }
.rich-editor-actions small { color: #929bab; font-size: 10px; }
@media (max-width: 600px) { .image-strip { grid-template-columns: 1fr; } .rich-editor-actions { align-items: flex-start; flex-direction: column; } }
</style>
