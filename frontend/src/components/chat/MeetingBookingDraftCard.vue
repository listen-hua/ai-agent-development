<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Calendar, Search, UserFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { meetingService } from '@/services/meeting'
import type { MeetingAttendeeOption, MeetingBookingDraft, MeetingBookingDraftResult } from '@/types/domain'

const props = defineProps<{ draft: MeetingBookingDraft }>()
const emit = defineEmits<{ updated: [result: MeetingBookingDraftResult] }>()
const auth = useAuthStore()
const title = ref(props.draft.slots.title || '')
const selectedUserIDs = ref<string[]>([...(props.draft.slots.attendee_user_ids || [])])
const selectedBookingID = ref(props.draft.slots.selected_booking_id || '')
const options = ref<MeetingAttendeeOption[]>([...(props.draft.attendee_choices || [])])
const searching = ref(false)
const submitting = ref(false)
const active = computed(() => props.draft.status === 'collecting' && new Date(props.draft.expires_at).getTime() > Date.now())

watch(() => props.draft, (value) => {
  title.value = value.slots.title || ''
  selectedUserIDs.value = [...(value.slots.attendee_user_ids || [])]
  selectedBookingID.value = value.slots.selected_booking_id || ''
  options.value = [...(value.attendee_choices || []), ...options.value.filter((item) => !(value.attendee_choices || []).some((selected) => selected.id === item.id))]
}, { deep: true })

async function searchPeople(query = '') {
  searching.value = true
  try {
    const values = await meetingService.searchAttendees(query)
    const selected = options.value.filter((item) => selectedUserIDs.value.includes(item.id))
    options.value = [...selected, ...values.filter((item) => !selected.some((selectedItem) => selectedItem.id === item.id))]
  } finally {
    searching.value = false
  }
}

async function submit(selfOnly = false) {
  if (!active.value || submitting.value) return
  if (props.draft.intent === 'cancel' && !selectedBookingID.value) {
    ElMessage.warning('请选择要取消的会议')
    return
  }
  if (props.draft.intent !== 'cancel' && !title.value.trim()) {
    ElMessage.warning('请填写会议主题')
    return
  }
  submitting.value = true
  try {
    const result = await meetingService.updateDraft(props.draft.id, props.draft.intent === 'cancel'
      ? { attendee_user_ids: [], booking_id: selectedBookingID.value, version: props.draft.version }
      : { title: title.value.trim(), attendee_user_ids: selfOnly ? [] : selectedUserIDs.value, attendees_confirmed: true, version: props.draft.version })
    emit('updated', result)
    ElMessage.success(result.action ? '信息已补充，请确认会议室方案' : '会议信息已保存')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存会议信息失败')
  } finally {
    submitting.value = false
  }
}

function formatBooking(startAt: string, endAt: string) {
  const start = new Date(startAt)
  const end = new Date(endAt)
  return `${start.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })}–${end.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })}`
}
</script>

<template>
  <section class="meeting-draft-card" :class="draft.status">
    <header>
      <span class="draft-icon"><Calendar /></span>
      <div><strong>{{ draft.intent === 'cancel' ? '选择要取消的会议' : '补充会议信息' }}</strong><small>草稿将在 {{ new Date(draft.expires_at).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) }} 失效</small></div>
    </header>

    <template v-if="draft.intent === 'cancel'">
      <el-radio-group v-model="selectedBookingID" class="booking-choice-list" :disabled="!active">
        <el-radio v-for="booking in draft.booking_choices || []" :key="booking.id" :value="booking.id" border>
          <strong>{{ booking.title }}</strong>
          <span>{{ formatBooking(booking.start_at, booking.end_at) }} · {{ booking.room_name }}</span>
        </el-radio>
      </el-radio-group>
      <el-button type="danger" :loading="submitting" :disabled="!active || !selectedBookingID" @click="submit()">生成取消确认</el-button>
    </template>

    <template v-else>
      <label class="field-label">会议主题</label>
      <el-input v-model="title" maxlength="100" show-word-limit placeholder="例如：产品周报评审" :disabled="!active" />
      <label class="field-label">邀请飞书同事</label>
      <div class="requester-chip"><el-avatar :size="28" :src="auth.user?.avatar_url"><UserFilled /></el-avatar><span><strong>{{ auth.user?.name || '我' }}</strong><small>申请人 · 自动加入</small></span></div>
      <el-select
        v-model="selectedUserIDs"
        multiple
        filterable
        remote
        reserve-keyword
        :remote-method="searchPeople"
        :loading="searching"
        :disabled="!active"
        placeholder="输入姓名或职位搜索"
        class="attendee-select"
        @visible-change="(visible: boolean) => visible && !options.length && searchPeople()"
      >
        <template #prefix><Search /></template>
        <el-option v-for="person in options" :key="person.id" :label="person.name" :value="person.id">
          <div class="person-option"><el-avatar :size="26" :src="person.avatar_url" /><span><strong>{{ person.name }}</strong><small>{{ [...(person.department_names || []), person.job_title].filter(Boolean).join(' · ') || '员工' }}</small></span></div>
        </el-option>
      </el-select>
      <footer>
        <el-button :disabled="!active" :loading="submitting" @click="submit(true)">仅自己参会</el-button>
        <el-button type="primary" :disabled="!active" :loading="submitting" @click="submit(false)">继续查找会议室</el-button>
      </footer>
    </template>
  </section>
</template>

<style scoped>
.meeting-draft-card{margin-top:14px;padding:16px;border:1px solid rgba(119,91,255,.22);border-radius:16px;background:linear-gradient(145deg,rgba(250,248,255,.98),rgba(245,249,255,.98));box-shadow:0 12px 30px rgba(72,54,142,.08)}
.meeting-draft-card>header{display:flex;align-items:center;gap:10px;margin-bottom:14px}.meeting-draft-card header div{display:flex;flex-direction:column;gap:3px}.meeting-draft-card header small{color:#8a849d}.draft-icon{display:grid;place-items:center;width:36px;height:36px;border-radius:11px;color:#7357e8;background:#ebe6ff}.field-label{display:block;margin:12px 0 7px;font-size:13px;font-weight:700;color:#4f4961}.requester-chip{display:flex;align-items:center;gap:9px;margin-bottom:8px;padding:8px 10px;border-radius:11px;background:rgba(115,87,232,.08)}.requester-chip span,.person-option span{display:flex;flex-direction:column}.requester-chip small,.person-option small{font-size:11px;color:#918aa0}.attendee-select{width:100%}.person-option{display:flex;align-items:center;gap:8px}.meeting-draft-card footer{display:flex;justify-content:flex-end;gap:8px;margin-top:14px}.booking-choice-list{display:flex;flex-direction:column;align-items:stretch;width:100%;margin-bottom:14px}.booking-choice-list :deep(.el-radio){height:auto;margin:0 0 8px;padding:12px}.booking-choice-list :deep(.el-radio__label){display:flex;flex-direction:column;gap:4px}.booking-choice-list span{font-size:12px;color:#777083}.meeting-draft-card.completed,.meeting-draft-card.expired{opacity:.72}
</style>
