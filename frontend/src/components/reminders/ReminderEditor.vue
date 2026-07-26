<script setup lang="ts">
import { reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { Reminder, ReminderSchedule, ReminderScheduleType } from '@/types/domain'

const props = defineProps<{ modelValue: boolean; reminder?: Reminder; saving: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; save: [value: { content: string; schedule: ReminderSchedule }] }>()
const form = reactive<{ content: string; type: ReminderScheduleType; once_at: string; local_time: string; weekdays: number[] }>({ content: '', type: 'once', once_at: '', local_time: '09:00', weekdays: [1] })
const weekdayOptions = [{ value: 1, label: '周一' },{ value: 2, label: '周二' },{ value: 3, label: '周三' },{ value: 4, label: '周四' },{ value: 5, label: '周五' },{ value: 6, label: '周六' },{ value: 7, label: '周日' }]

watch(() => props.modelValue, (open) => {
  if (!open) return
  const reminder = props.reminder
  form.content = reminder?.content || ''
  form.type = reminder?.schedule.type || 'once'
  form.once_at = reminder?.schedule.once_at || ''
  form.local_time = reminder?.schedule.local_time || '09:00'
  form.weekdays = [...(reminder?.schedule.weekdays || [1])]
})

function submit() {
  if (!form.content.trim()) return ElMessage.warning('请输入提醒内容')
  if (form.type === 'once' && !form.once_at) return ElMessage.warning('请选择提醒时间')
  if (form.type !== 'once' && !form.local_time) return ElMessage.warning('请选择每天的提醒时间')
  if (form.type === 'weekly' && !form.weekdays.length) return ElMessage.warning('至少选择一个星期')
  const schedule: ReminderSchedule = { type: form.type, timezone: 'Asia/Shanghai' }
  if (form.type === 'once') schedule.once_at = form.once_at
  else schedule.local_time = form.local_time
  if (form.type === 'weekly') schedule.weekdays = [...form.weekdays].sort()
  emit('save', { content: form.content.trim(), schedule })
}
</script>

<template>
  <el-dialog :model-value="modelValue" :title="reminder ? '修改提醒' : '新建提醒'" width="520px" destroy-on-close @update:model-value="emit('update:modelValue', $event)">
    <el-form label-position="top" class="reminder-editor">
      <el-form-item label="提醒内容"><el-input v-model="form.content" type="textarea" :rows="3" maxlength="500" show-word-limit placeholder="例如：写周报" /></el-form-item>
      <el-form-item label="重复规则"><el-segmented v-model="form.type" :options="[{ label: '单次', value: 'once' }, { label: '每天', value: 'daily' }, { label: '工作日', value: 'workday' }, { label: '每周', value: 'weekly' }]" /></el-form-item>
      <el-form-item v-if="form.type === 'once'" label="提醒时间"><el-date-picker v-model="form.once_at" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" format="YYYY-MM-DD HH:mm" placeholder="选择日期和时间" /></el-form-item>
      <el-form-item v-else label="提醒时刻"><el-time-picker v-model="form.local_time" value-format="HH:mm" format="HH:mm" /></el-form-item>
      <el-form-item v-if="form.type === 'weekly'" label="星期"><el-checkbox-group v-model="form.weekdays"><el-checkbox-button v-for="item in weekdayOptions" :key="item.value" :value="item.value">{{ item.label }}</el-checkbox-button></el-checkbox-group></el-form-item>
      <div class="reminder-editor-tip">所有时间按北京时间解释；保存后仍需再次确认才会生效。</div>
    </el-form>
    <template #footer><el-button @click="emit('update:modelValue', false)">取消</el-button><el-button type="primary" :loading="saving" @click="submit">预览并确认</el-button></template>
  </el-dialog>
</template>

<style scoped>
.reminder-editor :deep(.el-segmented){width:100%}.reminder-editor :deep(.el-segmented__item){flex:1}.reminder-editor :deep(.el-date-editor){width:100%}.reminder-editor-tip{border-radius:9px;padding:10px 12px;background:#f2f5ff;color:#65739a;font-size: 13px;line-height:1.6}
</style>
