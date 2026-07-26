<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Bell, Plus } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import PageHeader from '@/components/common/PageHeader.vue'
import ReminderEditor from '@/components/reminders/ReminderEditor.vue'
import ReminderTable from '@/components/reminders/ReminderTable.vue'
import { reminderService } from '@/services/reminder'
import type { Reminder, ReminderActionDraft, ReminderSchedule } from '@/types/domain'
import { formatReminderSchedule } from '@/utils/reminder'

const items = ref<Reminder[]>([])
const loading = ref(false)
const saving = ref(false)
const editorOpen = ref(false)
const editing = ref<Reminder>()
const activeCount = computed(() => items.value.filter((item) => item.status === 'active').length)
const pausedCount = computed(() => items.value.filter((item) => item.status === 'paused').length)
const failedCount = computed(() => items.value.filter((item) => item.status === 'failed').length)

async function load() { loading.value = true; try { items.value = await reminderService.list() } finally { loading.value = false } }
function create() { editing.value = undefined; editorOpen.value = true }
function edit(reminder: Reminder) { editing.value = reminder; editorOpen.value = true }

async function confirmDraft(draft: ReminderActionDraft, description: string) {
  try {
    await ElMessageBox.confirm(description, '确认提醒操作', { type: 'warning', confirmButtonText: '确认生效', cancelButtonText: '取消' })
  } catch {
    await reminderService.cancelAction(draft.id).catch(() => undefined)
    return false
  }
  try {
    await reminderService.confirm(draft.id)
    ElMessage.success('提醒操作已生效')
    return true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '提醒操作确认失败，请稍后重试')
    return false
  }
}

async function save(value: { content: string; schedule: ReminderSchedule }) {
  saving.value = true
  try {
    const draft = editing.value ? await reminderService.createAction(editing.value.id, { action: 'update', ...value }) : await reminderService.create(value)
    const confirmed = await confirmDraft(draft, `内容：${value.content}\n时间：${formatReminderSchedule(value.schedule)}\n\n确认后才会正式生效。`)
    if (confirmed) { editorOpen.value = false; await load() }
  } finally { saving.value = false }
}

async function action(reminder: Reminder, actionName: 'pause' | 'resume' | 'delete') {
  const labels = { pause: '暂停', resume: '恢复', delete: '删除' }
  const draft = await reminderService.createAction(reminder.id, { action: actionName })
  if (await confirmDraft(draft, `确认${labels[actionName]}提醒“${reminder.content}”吗？`)) await load()
}
onMounted(load)
</script>

<template>
  <section class="admin-page reminder-page">
    <PageHeader eyebrow="PERSONAL AUTOMATION" title="我的提醒" description="通过飞书机器人私聊投递；创建、修改和删除都必须由你本人确认。"><el-button type="primary" :icon="Plus" @click="create">新建提醒</el-button></PageHeader>
    <div class="reminder-summary"><article><span><Bell /></span><div><strong>{{ activeCount }}</strong><small>生效中的提醒</small></div></article><article><div><strong>{{ pausedCount }}</strong><small>已暂停</small></div></article><article :class="{ danger: failedCount }"><div><strong>{{ failedCount }}</strong><small>需要处理</small></div></article><p>时间统一按 Asia/Shanghai 解释。工作日提醒会遵循管理员维护的公司工作日历。</p></div>
    <div class="section-heading"><div><h2>提醒任务</h2><p>最多保留 100 个有效提醒，历史状态保留 90 天。</p></div></div>
    <ReminderTable :items="items" :loading="loading" @edit="edit" @action="action" />
    <ReminderEditor v-model="editorOpen" :reminder="editing" :saving="saving" @save="save" />
  </section>
</template>

<style scoped>
.reminder-summary{display:grid;grid-template-columns:180px 150px 150px minmax(220px,1fr);gap:12px;align-items:stretch}.reminder-summary article,.reminder-summary>p{min-height:84px;border:1px solid #e5e9f0;border-radius:13px;padding:16px 18px;background:#fff}.reminder-summary article{display:flex;align-items:center;gap:12px}.reminder-summary article>span{width:38px;height:38px;border-radius:11px;background:#edf1ff;color:#526ed9;display:grid;place-items:center}.reminder-summary article div{display:flex;flex-direction:column}.reminder-summary strong{color:#26334b;font-size:24px}.reminder-summary small{color:#9099a9;font-size: 12px}.reminder-summary article.danger strong{color:#d04a4a}.reminder-summary>p{margin:0;color:#6d7890;font-size: 13px;line-height:1.7;background:linear-gradient(120deg,#f4f7ff,#fff)}
@media(max-width:900px){.reminder-summary{grid-template-columns:repeat(3,1fr)}.reminder-summary>p{grid-column:1/-1}}@media(max-width:600px){.reminder-summary{grid-template-columns:1fr 1fr}.reminder-summary>p{grid-column:1/-1}.reminder-summary article:first-child{grid-column:1/-1}}
</style>
