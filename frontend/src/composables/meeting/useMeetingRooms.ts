import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { meetingRoomAdminService } from '@/services/meeting'
import type { MeetingBooking, MeetingRoom, MeetingSettings } from '@/types/domain'

export function useMeetingRooms() {
  const rooms = ref<MeetingRoom[]>([])
  const bookingAlerts = ref<MeetingBooking[]>([])
  const settings = ref<MeetingSettings>()
  const loading = ref(false)
  const syncing = ref(false)
  const initializing = ref(false)
  const ordinaryCount = computed(() => rooms.value.filter((room) => room.enabled && room.schedule_enabled && !room.last_error && room.approval_switch === 0).length)
  const restrictedCount = computed(() => rooms.value.length - ordinaryCount.value)
  const alertCount = computed(() => rooms.value.filter((room) => room.last_error || !room.enabled || !room.schedule_enabled).length + bookingAlerts.value.length)

  async function load() {
    loading.value = true
    try {
      const value = await meetingRoomAdminService.load()
      rooms.value = value.rooms
      bookingAlerts.value = value.booking_alerts || []
      settings.value = value.settings
    } finally { loading.value = false }
  }

  async function sync() {
    syncing.value = true
    try {
      rooms.value = await meetingRoomAdminService.sync()
      ElMessage.success(`已同步 ${rooms.value.length} 间会议室及预定限制`)
      await load()
    } finally { syncing.value = false }
  }

  async function initializeCalendar() {
    initializing.value = true
    try {
      settings.value = await meetingRoomAdminService.initializeCalendar()
      ElMessage.success('行政 AI 共享日历已初始化')
    } finally { initializing.value = false }
  }

  return { rooms, bookingAlerts, settings, loading, syncing, initializing, ordinaryCount, restrictedCount, alertCount, load, sync, initializeCalendar }
}
