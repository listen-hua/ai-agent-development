<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { EditPen } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import PageHeader from '@/components/common/PageHeader.vue'
import NotificationEditor from '@/components/notifications/NotificationEditor.vue'
import NotificationTable from '@/components/notifications/NotificationTable.vue'
import { notificationService } from '@/services/admin'
import type { NotificationCreateInput, NotificationDraft, NotificationTargetOption } from '@/types/domain'

const items = ref<NotificationDraft[]>([])
const users = ref<NotificationTargetOption[]>([])
const chats = ref<NotificationTargetOption[]>([])
const chatError = ref('')
const editorOpen = ref(false)
const saving = ref(false)
const drafting = ref(false)
const uploading = ref(false)
const editor = ref<InstanceType<typeof NotificationEditor>>()

async function load() { items.value = await notificationService.list() }
async function loadTargets() {
  const result = await notificationService.targets()
  users.value = result.users || []
  chats.value = result.chats || []
  chatError.value = result.chat_error || ''
}
async function openEditor() {
  editorOpen.value = true
  try {
    await loadTargets()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '读取接收目标失败')
  }
}
async function create(value: NotificationCreateInput) {
  saving.value = true
  try {
    await notificationService.create(value)
    editorOpen.value = false
    ElMessage.success('通知已保存为草稿，审核前不会发送')
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存通知失败')
  } finally {
    saving.value = false
  }
}
async function approve(id: string) {
  await ElMessageBox.confirm('请确认通知正文、图片和接收目标均已人工复核。计划通知在审核后会自动进入待发送队列。', '审核通知', { type: 'warning' })
  const result = await notificationService.approve(id)
  ElMessage.success(result.status === 'scheduled' ? '审核通过，已进入计划发送队列' : '审核通过，可手动发送')
  await load()
}
async function send(id: string) {
  await ElMessageBox.confirm('将立即通过企业自建应用投递到已选个人或群聊，确认发送？', '正式发送', { type: 'warning', confirmButtonText: '确认发送' })
  const result = await notificationService.send(id)
  if (result.status === 'sent') ElMessage.success('通知已发送')
  else ElMessage.warning(`通知发送状态：${result.status}${result.last_error ? `，${result.last_error}` : ''}`)
  await load()
}
async function cancel(id: string) {
  await notificationService.cancel(id)
  ElMessage.success('通知已取消')
  await load()
}
async function aiDraft(brief: string) {
  drafting.value = true
  try {
    const result = await notificationService.aiDraft(brief)
    editor.value?.applyDraft(result.content)
    ElMessage.success('AI 已润色，请继续人工核对事实')
  } finally {
    drafting.value = false
  }
}
async function uploadImage(file: File) {
  if (file.size > 10 * 1024 * 1024) {
    ElMessage.warning('单张图片不能超过 10 MB')
    return
  }
  const previewUrl = URL.createObjectURL(file)
  uploading.value = true
  try {
    const image = await notificationService.uploadImage(file)
    editor.value?.addImage({ ...image, preview_url: previewUrl })
    ElMessage.success('图片已上传到飞书')
  } catch (error) {
    URL.revokeObjectURL(previewUrl)
    ElMessage.error(error instanceof Error ? error.message : '图片上传失败')
  } finally {
    uploading.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="admin-page">
    <PageHeader eyebrow="HUMAN-IN-THE-LOOP" title="飞书通知中心" description="选择个人或群聊，编辑带图片的富文本卡片，并在人工审核后立即或定时发送。">
      <el-button type="primary" :icon="EditPen" @click="openEditor">起草通知</el-button>
    </PageHeader>
    <div class="guardrail-banner">
      <div class="guardrail-icon">审</div>
      <div><strong>人工审核护栏已开启</strong><p>任何草稿都不能跳过审核；计划通知仅在审核通过后进入发送队列，投递使用幂等键避免重复。</p></div>
      <el-tag type="success" effect="dark" round>强制启用</el-tag>
    </div>
    <div class="section-heading">
      <div><h2>通知任务</h2><p>查看接收目标、计划时间、审核状态和飞书投递结果。</p></div>
    </div>
    <NotificationTable :items="items" @approve="approve" @send="send" @cancel="cancel" />
    <NotificationEditor
      ref="editor"
      v-model="editorOpen"
      :saving="saving"
      :drafting="drafting"
      :uploading="uploading"
      :users="users"
      :chats="chats"
      :chat-error="chatError"
      @save="create"
      @ai-draft="aiDraft"
      @upload-image="uploadImage"
    />
  </section>
</template>
