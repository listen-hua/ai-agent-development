<script setup lang="ts">
import { reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import NotificationAudiencePicker from './NotificationAudiencePicker.vue'
import NotificationPreview from './NotificationPreview.vue'
import NotificationRichEditor from './NotificationRichEditor.vue'
import type { NotificationCreateInput, NotificationImage, NotificationRecipientType, NotificationTargetOption } from '@/types/domain'

const props = defineProps<{
  modelValue: boolean
  saving: boolean
  drafting: boolean
  uploading: boolean
  users: NotificationTargetOption[]
  chats: NotificationTargetOption[]
  chatError?: string
}>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: [value: NotificationCreateInput]
  aiDraft: [brief: string]
  uploadImage: [file: File]
}>()

const form = reactive({
  title: '',
  content: '',
  recipientType: 'user' as NotificationRecipientType,
  recipientIds: [] as string[],
  images: [] as NotificationImage[],
  scheduledAt: '',
})

watch(() => props.modelValue, (open) => {
  if (!open) return
  for (const image of form.images) if (image.preview_url) URL.revokeObjectURL(image.preview_url)
  Object.assign(form, { title: '', content: '', recipientType: 'user', recipientIds: [], images: [], scheduledAt: '' })
})

const startOfToday = () => {
  const date = new Date()
  date.setHours(0, 0, 0, 0)
  return date
}
const disabledDate = (date: Date) => date.getTime() < startOfToday().getTime()

function submit() {
  if (!form.title.trim() || !form.content.trim()) {
    ElMessage.warning('请填写通知标题和正文')
    return
  }
  if (!form.recipientIds.length) {
    ElMessage.warning(`请至少选择一${form.recipientType === 'user' ? '名接收人' : '个接收群聊'}`)
    return
  }
  let scheduledAt: string | undefined
  if (form.scheduledAt) {
    const value = new Date(form.scheduledAt)
    if (Number.isNaN(value.getTime()) || value.getTime() < Date.now() - 30_000) {
      ElMessage.warning('计划发送时间不能早于当前时间')
      return
    }
    scheduledAt = value.toISOString()
  }
  emit('save', {
    title: form.title.trim(),
    content: form.content.trim(),
    recipient_type: form.recipientType,
    recipient_ids: form.recipientIds,
    images: form.images.map(({ image_key, name, alt }) => ({ image_key, name, alt })),
    scheduled_at: scheduledAt,
  })
}

function removeImage(index: number) {
  const [removed] = form.images.splice(index, 1)
  if (removed?.preview_url) URL.revokeObjectURL(removed.preview_url)
}
function applyDraft(value: string) { form.content = value }
function addImage(value: NotificationImage) { form.images.push(value) }
defineExpose({ applyDraft, addImage })
</script>

<template>
  <el-drawer
    :model-value="modelValue"
    title="起草行政通知"
    size="min(760px, 96vw)"
    :close-on-click-modal="!saving && !uploading"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <el-form class="notification-form" label-position="top">
      <el-form-item label="通知标题">
        <el-input v-model="form.title" maxlength="80" show-word-limit placeholder="例如：2026 年国庆放假安排" />
      </el-form-item>
      <el-form-item label="接收目标" required>
        <NotificationAudiencePicker
          v-model:recipient-type="form.recipientType"
          v-model:recipient-ids="form.recipientIds"
          :users="users"
          :chats="chats"
          :chat-error="chatError"
        />
      </el-form-item>
      <el-form-item label="通知正文" required>
        <NotificationRichEditor
          v-model="form.content"
          :images="form.images"
          :uploading="uploading"
          :drafting="drafting"
          @upload="emit('uploadImage', $event)"
          @remove-image="removeImage"
          @ai-draft="emit('aiDraft', $event)"
        />
      </el-form-item>
      <el-form-item label="计划发送时间（可选）">
        <el-date-picker
          v-model="form.scheduledAt"
          type="datetime"
          value-format="YYYY-MM-DDTHH:mm:ss"
          :disabled-date="disabledDate"
          placeholder="不选择则审核后手动发送"
          style="width: 100%"
        />
        <small class="schedule-tip">今天以前的日期不可选择；选择计划时间后，审核通过的通知将由 Worker 自动发送。</small>
      </el-form-item>
      <NotificationPreview :title="form.title" :content="form.content" :images="form.images" />
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="saving" :disabled="!form.title.trim() || !form.content.trim() || !form.recipientIds.length || uploading" @click="submit">保存草稿</el-button>
    </template>
  </el-drawer>
</template>

<style scoped>
.notification-form { padding: 0 3px 18px; }
.schedule-tip { display: block; margin-top: 7px; color: #8f98a8; font-size: 10px; line-height: 1.5; }
</style>
