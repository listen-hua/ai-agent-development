<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Calendar, Location, User } from '@element-plus/icons-vue'
import type { MeetingBookingAction } from '@/types/domain'

const props = defineProps<{ action: MeetingBookingAction }>()
const emit = defineEmits<{ confirm: [optionId?: string]; cancel: [] }>()
const selected = ref(props.action.selected_option_id || props.action.options[0]?.id || '')
const selectionLocked = computed(() => props.action.status !== 'pending')
const visibleOptions = computed(() => {
  if (!selectionLocked.value) return props.action.options
  const selectedId = props.action.selected_option_id || selected.value
  const selectedOption = props.action.options.find((option) => option.id === selectedId)
  return selectedOption ? [selectedOption] : props.action.options
})
watch(
  () => ({
    actionId: props.action.id,
    selectedId: props.action.selected_option_id,
    optionIds: props.action.options.map((option) => option.id).join(','),
  }),
  (value, previous) => {
    if (value.selectedId && props.action.options.some((option) => option.id === value.selectedId)) {
      selected.value = value.selectedId
      return
    }
    const selectionExists = props.action.options.some((option) => option.id === selected.value)
    if (!selectionExists || value.actionId !== previous?.actionId) {
      selected.value = props.action.options[0]?.id || ''
    }
  },
  { immediate: true },
)
const statusLabel = computed(() => ({ pending: '待确认', confirmed: '已确认', cancelled: '已取消', expired: '已过期', processing: '处理中' }[props.action.status]))
function formatRange(startAt: string, endAt: string) {
  const start = new Date(startAt); const end = new Date(endAt)
  return `${start.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })} ${start.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}–${end.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}`
}
</script>

<template>
  <section class="meeting-booking-card" :class="action.status">
    <header><div class="meeting-card-icon"><Calendar /></div><div><strong>{{ action.intent === 'cancel' ? '取消会议室预约' : action.intent === 'reschedule' ? '会议室改期确认' : '会议室预约确认' }}</strong><span>{{ action.title }}</span></div><el-tag size="small" :type="action.status === 'confirmed' ? 'success' : action.status === 'pending' ? 'primary' : 'info'">{{ statusLabel }}</el-tag></header>
    <template v-if="action.options.length">
      <el-radio-group v-model="selected" class="meeting-options" :disabled="selectionLocked" :aria-label="selectionLocked ? '已锁定的会议室' : '请选择会议室'">
        <el-radio v-for="option in visibleOptions" :key="option.id" :value="option.id" border>
          <span class="meeting-option-main"><b>{{ option.room_name }}</b><small><Calendar />{{ formatRange(option.start_at, option.end_at) }}</small></span>
          <span class="meeting-option-capacity"><User />{{ option.capacity }} 人</span>
        </el-radio>
      </el-radio-group>
      <div class="meeting-card-meta"><Location />{{ action.status === 'confirmed' ? '会议室已确认，不能再更换' : action.status === 'processing' ? '正在锁定所选会议室，请稍候' : '确认时会重新校验会议室与全部参会人忙闲' }}</div>
    </template>
    <div v-else class="meeting-card-meta"><Location />确认后将删除对应飞书日程并释放会议室</div>
    <footer v-if="action.status === 'pending'"><small>确认有效期至 {{ new Date(action.expires_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) }}</small><div><el-button size="small" @click="emit('cancel')">暂不操作</el-button><el-button type="primary" size="small" @click="emit('confirm', selected || undefined)">{{ action.intent === 'cancel' ? '确认取消预约' : action.intent === 'reschedule' ? '确认改期' : '确认预约' }}</el-button></div></footer>
  </section>
</template>
