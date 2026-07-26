import { api } from './api'
import type { MeetingBooking, MeetingBookingAction, MeetingBookingActionResult, MeetingRoom, MeetingRoomAdminData, MeetingSettings } from '@/types/domain'

export const meetingService = {
  confirmAction: (id: string, optionId?: string) => api.post<MeetingBookingActionResult>(`/api/v1/meeting-booking-actions/${id}/confirm`, { option_id: optionId || '' }),
  cancelAction: (id: string) => api.post<MeetingBookingAction>(`/api/v1/meeting-booking-actions/${id}/cancel`),
  listBookings: () => api.get<MeetingBooking[]>('/api/v1/meeting-bookings'),
}

export const meetingRoomAdminService = {
  load: () => api.get<MeetingRoomAdminData>('/api/v1/admin/meeting-rooms'),
  sync: () => api.post<MeetingRoom[]>('/api/v1/admin/meeting-rooms/sync'),
  initializeCalendar: () => api.post<MeetingSettings>('/api/v1/admin/meeting-rooms/calendar'),
}
