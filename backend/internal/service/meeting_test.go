package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/store"
)

type fakeMeetingFeishu struct {
	roomBusy         map[string][]feishu.BusyInterval
	userBusy         map[string][]feishu.BusyInterval
	roomRSVP         string
	listRoomsError   error
	addUsersError    error
	deleteError      error
	listRoomCalls    int
	createdEvents    int
	deletedEvents    int
	cleanupReminders int
	cleanupOpenID    string
	cleanupKey       string
}

func (f *fakeMeetingFeishu) Configured() bool { return true }
func (f *fakeMeetingFeishu) ListMeetingRooms(context.Context) ([]feishu.MeetingRoomInfo, error) {
	f.listRoomCalls++
	return nil, f.listRoomsError
}
func (f *fakeMeetingFeishu) GetMeetingRoomReserveConfig(context.Context, string) (feishu.MeetingRoomReserveConfig, error) {
	return feishu.MeetingRoomReserveConfig{}, nil
}
func (f *fakeMeetingFeishu) MeetingRoomFreeBusy(_ context.Context, ids []string, _, _ time.Time) (map[string][]feishu.BusyInterval, error) {
	values := map[string][]feishu.BusyInterval{}
	for _, id := range ids {
		values[id] = f.roomBusy[id]
	}
	return values, nil
}
func (f *fakeMeetingFeishu) UserFreeBusy(_ context.Context, ids []string, _, _ time.Time) (map[string][]feishu.BusyInterval, error) {
	values := map[string][]feishu.BusyInterval{}
	for _, id := range ids {
		values[id] = f.userBusy[id]
	}
	return values, nil
}
func (f *fakeMeetingFeishu) CreateSharedCalendar(context.Context, string) (string, error) {
	return "calendar", nil
}
func (f *fakeMeetingFeishu) CreateCalendarEvent(context.Context, string, string, string, time.Time, time.Time, string) (feishu.CalendarEventInfo, error) {
	f.createdEvents++
	return feishu.CalendarEventInfo{EventID: "event-1"}, nil
}
func (f *fakeMeetingFeishu) AddCalendarEventAttendees(_ context.Context, _, _ string, attendees []feishu.EventAttendeeInput, _ bool) ([]feishu.EventAttendee, error) {
	if len(attendees) > 0 && attendees[0].Type == "resource" {
		status := f.roomRSVP
		if status == "" {
			status = "accept"
		}
		return []feishu.EventAttendee{{Type: "resource", RoomID: attendees[0].RoomID, RSVPStatus: status}}, nil
	}
	if f.addUsersError != nil {
		return nil, f.addUsersError
	}
	return nil, nil
}
func (f *fakeMeetingFeishu) ListCalendarEventAttendees(context.Context, string, string) ([]feishu.EventAttendee, error) {
	return nil, nil
}
func (f *fakeMeetingFeishu) DeleteCalendarEvent(context.Context, string, string, bool) error {
	f.deletedEvents++
	return f.deleteError
}
func (f *fakeMeetingFeishu) SendMeetingBookingResult(context.Context, string, string, string, string, string) (string, error) {
	return "message", nil
}
func (f *fakeMeetingFeishu) SendMeetingRoomCleanupReminder(_ context.Context, openID, _ string, idempotencyKey string) (string, error) {
	f.cleanupReminders++
	f.cleanupOpenID = openID
	f.cleanupKey = idempotencyKey
	return "message", nil
}

func testMeetingService(t *testing.T, fake *fakeMeetingFeishu) (*Meeting, *store.Memory, domain.User, time.Time) {
	t.Helper()
	repo := store.NewMemory(domain.AgentConfig{})
	user, _ := repo.GetUser(context.Background(), store.DemoEmployeeID)
	base := time.Date(2026, 7, 20, 10, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600))
	service := NewMeeting(repo, fake, NewLocalSlotLocker(), "Asia/Shanghai", "calendar")
	service.now = func() time.Time { return base }
	settings := domain.MeetingSettings{CalendarID: "calendar", Timezone: "Asia/Shanghai", WorkdayStart: "09:00", WorkdayEnd: "18:00", SlotMinutes: 30, SyncIntervalMinute: 15, UpdatedAt: base}
	if err := repo.SaveMeetingSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	rooms := []domain.MeetingRoom{
		{ID: "00000000-0000-4000-8000-000000000201", RoomID: "room-small", Name: "海棠", Capacity: 4, Enabled: true, ScheduleEnabled: true, ReservationStartSeconds: 9 * 3600, ReservationEndSeconds: 18 * 3600, MaxDurationHours: 2, SyncedAt: base},
		{ID: "00000000-0000-4000-8000-000000000202", RoomID: "room-large", Name: "远山", Capacity: 8, Enabled: true, ScheduleEnabled: true, ReservationStartSeconds: 9 * 3600, ReservationEndSeconds: 18 * 3600, MaxDurationHours: 2, SyncedAt: base},
	}
	if err := repo.UpsertMeetingRooms(context.Background(), rooms); err != nil {
		t.Fatal(err)
	}
	return service, repo, user, base
}

func TestParseMeetingRequestRelativeDateAndDefaultDuration(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*3600)
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, location)
	request, clarification := parseMeetingRequest("预约明天下午3点会议室，参会人张三、李四", now, location)
	if clarification != "" {
		t.Fatal(clarification)
	}
	if request.Date.Day() != 21 || len(request.Times) != 1 || request.Times[0].Hour != 15 || request.Duration != 30*time.Minute {
		t.Fatalf("unexpected request: %#v", request)
	}
	if len(request.AttendeeNames) != 2 {
		t.Fatalf("unexpected attendees: %#v", request.AttendeeNames)
	}
}

func TestParseMeetingRequestUsesExplicitStartAndEndAsRange(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*3600)
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, location)
	request, clarification := parseMeetingRequest("帮我看下1号会议室有没有人，没人帮我预约下，时间是今天的，从16点30开始，预计17点结束", now, location)
	if clarification != "" {
		t.Fatal(clarification)
	}
	if len(request.Times) != 1 || request.Times[0] != (clockTime{Hour: 16, Minute: 30}) {
		t.Fatalf("expected 16:30 as the only start time, got %#v", request.Times)
	}
	if request.Duration != 30*time.Minute {
		t.Fatalf("expected a 30 minute range, got %s", request.Duration)
	}
}

func TestRequestedMeetingRoomNameAcceptsRoomTypeAlias(t *testing.T) {
	rooms := []domain.MeetingRoom{
		{Name: "1号洽谈室"},
		{Name: "3号会议室"},
	}
	if got := requestedMeetingRoomName("帮我预约1号会议室", rooms); got != "1号洽谈室" {
		t.Fatalf("expected room alias to resolve uniquely, got %q", got)
	}
}

func TestMeetingCandidatesCheckRoomBusyAndCapacity(t *testing.T) {
	fake := &fakeMeetingFeishu{roomBusy: map[string][]feishu.BusyInterval{}, userBusy: map[string][]feishu.BusyInterval{}}
	meeting, _, user, base := testMeetingService(t, fake)
	start := base.AddDate(0, 0, 1).Add(5 * time.Hour)
	fake.roomBusy["room-small"] = []feishu.BusyInterval{{Start: start, End: start.Add(30 * time.Minute)}}
	request, _ := parseMeetingRequest("明天下午3点预约4人会议室", base, meeting.location)
	attendees := []domain.MeetingAttendee{{UserID: user.ID, OpenID: user.FeishuOpenID, Name: user.Name}}
	options, _, err := meeting.candidates(context.Background(), request, "明天下午3点预约4人会议室", attendees)
	if err != nil {
		t.Fatal(err)
	}
	if len(options) == 0 || options[0].RoomID != "room-large" || !options[0].StartAt.Equal(start) {
		t.Fatalf("unexpected options: %#v", options)
	}
}

func TestMeetingConfirmCreatesEventRoomUsersAndReminder(t *testing.T) {
	fake := &fakeMeetingFeishu{roomBusy: map[string][]feishu.BusyInterval{}, userBusy: map[string][]feishu.BusyInterval{}, roomRSVP: "accept"}
	meeting, repo, user, base := testMeetingService(t, fake)
	option := domain.MeetingBookingOption{ID: "00000000-0000-4000-8000-000000000301", RoomID: "room-small", RoomName: "海棠", Capacity: 4, StartAt: base.AddDate(0, 0, 1).Add(5 * time.Hour), EndAt: base.AddDate(0, 0, 1).Add(5*time.Hour + 30*time.Minute)}
	action := domain.MeetingBookingAction{ID: "00000000-0000-4000-8000-000000000302", UserID: user.ID, Intent: "create", Title: "周报会", Attendees: []domain.MeetingAttendee{{UserID: user.ID, OpenID: user.FeishuOpenID, Name: user.Name}}, Capacity: 1, Options: []domain.MeetingBookingOption{option}, Status: domain.MeetingActionPending, SourceChannel: "h5", ExpiresAt: base.Add(15 * time.Minute), CreatedAt: base}
	if err := repo.CreateMeetingBookingAction(context.Background(), action); err != nil {
		t.Fatal(err)
	}
	confirmed, booking, err := meeting.Confirm(context.Background(), user, action.ID, option.ID)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Status != domain.MeetingActionConfirmed || booking == nil || booking.Status != "active" || fake.createdEvents != 1 {
		t.Fatalf("unexpected result: %#v %#v", confirmed, booking)
	}
	meeting.now = func() time.Time { return option.EndAt.Add(time.Second) }
	if err = NewMeetingDispatcher(repo, meeting).Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.cleanupReminders != 1 || fake.cleanupOpenID != user.FeishuOpenID {
		t.Fatalf("expected one cleanup reminder for the organizer, got calls=%d open_id=%q", fake.cleanupReminders, fake.cleanupOpenID)
	}
	if len(fake.cleanupKey) > 50 || fake.cleanupKey != meetingCleanupIdempotencyKey(booking.ID) {
		t.Fatalf("invalid cleanup reminder idempotency key %q", fake.cleanupKey)
	}
}

func TestMeetingCleanupIdempotencyKeyFitsFeishuLimit(t *testing.T) {
	key := meetingCleanupIdempotencyKey("7fe9b0d6-91b1-48a3-a453-2ef9765207a9")
	if len(key) > 50 {
		t.Fatalf("idempotency key exceeds Feishu's 50 character limit: %d", len(key))
	}
	if key != "mtg-clean-7fe9b0d691b148a3a4532ef9765207a9" {
		t.Fatalf("unexpected idempotency key: %q", key)
	}
}

func TestMeetingConfirmCompensatesWhenAddingUsersFails(t *testing.T) {
	fake := &fakeMeetingFeishu{roomBusy: map[string][]feishu.BusyInterval{}, userBusy: map[string][]feishu.BusyInterval{}, roomRSVP: "accept", addUsersError: errors.New("attendee failure")}
	meeting, repo, user, base := testMeetingService(t, fake)
	option := domain.MeetingBookingOption{ID: "00000000-0000-4000-8000-000000000401", RoomID: "room-small", RoomName: "海棠", Capacity: 4, StartAt: base.AddDate(0, 0, 1).Add(5 * time.Hour), EndAt: base.AddDate(0, 0, 1).Add(5*time.Hour + 30*time.Minute)}
	action := domain.MeetingBookingAction{ID: "00000000-0000-4000-8000-000000000402", UserID: user.ID, Intent: "create", Title: "周报会", Attendees: []domain.MeetingAttendee{{UserID: user.ID, OpenID: user.FeishuOpenID, Name: user.Name}}, Capacity: 1, Options: []domain.MeetingBookingOption{option}, Status: domain.MeetingActionPending, SourceChannel: "h5", ExpiresAt: base.Add(15 * time.Minute), CreatedAt: base}
	if err := repo.CreateMeetingBookingAction(context.Background(), action); err != nil {
		t.Fatal(err)
	}
	if _, _, err := meeting.Confirm(context.Background(), user, action.ID, option.ID); err == nil {
		t.Fatal("expected attendee failure")
	}
	if fake.deletedEvents != 1 {
		t.Fatalf("temporary event was not deleted: %d", fake.deletedEvents)
	}
	stored, _ := repo.GetMeetingBookingAction(context.Background(), action.ID)
	if stored.Status != domain.MeetingActionPending {
		t.Fatalf("expected retryable action, got %s", stored.Status)
	}
}

func TestMeetingChatExplainsPastTime(t *testing.T) {
	fake := &fakeMeetingFeishu{roomBusy: map[string][]feishu.BusyInterval{}, userBusy: map[string][]feishu.BusyInterval{}}
	meeting, _, user, _ := testMeetingService(t, fake)
	handled, message, err := meeting.HandleChat(context.Background(), user, "", "今天上午9点预约会议室", "h5")
	if err != nil || !handled {
		t.Fatalf("expected handled user validation, handled=%v err=%v", handled, err)
	}
	if message.Content != "会议开始时间必须晚于当前时间，请重新指定今天稍晚的时间或其他日期。" {
		t.Fatalf("unexpected message: %s", message.Content)
	}
}

func TestMeetingCleanupFailureMakesActionTerminal(t *testing.T) {
	fake := &fakeMeetingFeishu{roomBusy: map[string][]feishu.BusyInterval{}, userBusy: map[string][]feishu.BusyInterval{}, roomRSVP: "accept", addUsersError: errors.New("attendee failure"), deleteError: errors.New("cleanup failure")}
	meeting, repo, user, base := testMeetingService(t, fake)
	option := domain.MeetingBookingOption{ID: "00000000-0000-4000-8000-000000000501", RoomID: "room-small", RoomName: "海棠", Capacity: 4, StartAt: base.AddDate(0, 0, 1).Add(5 * time.Hour), EndAt: base.AddDate(0, 0, 1).Add(5*time.Hour + 30*time.Minute)}
	action := domain.MeetingBookingAction{ID: "00000000-0000-4000-8000-000000000502", UserID: user.ID, Intent: "create", Title: "周报会", Attendees: []domain.MeetingAttendee{{UserID: user.ID, OpenID: user.FeishuOpenID, Name: user.Name}}, Capacity: 1, Options: []domain.MeetingBookingOption{option}, Status: domain.MeetingActionPending, SourceChannel: "h5", ExpiresAt: base.Add(15 * time.Minute), CreatedAt: base}
	if err := repo.CreateMeetingBookingAction(context.Background(), action); err != nil {
		t.Fatal(err)
	}
	if _, _, err := meeting.Confirm(context.Background(), user, action.ID, option.ID); err == nil {
		t.Fatal("expected attendee failure")
	}
	stored, _ := repo.GetMeetingBookingAction(context.Background(), action.ID)
	if stored.Status != domain.MeetingActionExpired || stored.ResultBookingID == "" {
		t.Fatalf("expected terminal action linked to needs_admin booking, got %#v", stored)
	}
}

func TestMeetingDispatcherThrottlesFailedRoomSync(t *testing.T) {
	fake := &fakeMeetingFeishu{listRoomsError: errors.New("permission denied")}
	meeting, repo, _, _ := testMeetingService(t, fake)
	dispatcher := NewMeetingDispatcher(repo, meeting)
	if err := dispatcher.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fake.listRoomCalls != 1 {
		t.Fatalf("expected one sync attempt within interval, got %d", fake.listRoomCalls)
	}
}

func TestMeetingChatCreatesDraftUntilTitleAndAttendeesAreConfirmed(t *testing.T) {
	fake := &fakeMeetingFeishu{roomBusy: map[string][]feishu.BusyInterval{}, userBusy: map[string][]feishu.BusyInterval{}}
	meeting, _, user, _ := testMeetingService(t, fake)
	handled, message, err := meeting.HandleChat(context.Background(), user, "conversation-1", "明天下午3点预约会议室", "h5")
	if err != nil || !handled || message.MeetingBookingDraft == nil {
		t.Fatalf("expected a booking draft, handled=%v message=%#v err=%v", handled, message, err)
	}
	draft := message.MeetingBookingDraft
	if len(draft.MissingFields) != 2 || draft.MissingFields[0] != "title" || draft.MissingFields[1] != "attendees" {
		t.Fatalf("unexpected missing fields: %#v", draft.MissingFields)
	}
	title := "产品周报评审"
	confirmed := true
	result, err := meeting.UpdateDraft(context.Background(), user, draft.ID, MeetingBookingDraftUpdate{Title: &title, AttendeesConfirmed: &confirmed, AttendeeUserIDs: []string{}, Version: draft.Version})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action == nil || result.Action.Title != title || len(result.Action.Attendees) != 1 || result.Action.Attendees[0].UserID != user.ID {
		t.Fatalf("unexpected completed draft result: %#v", result)
	}
	if _, err = meeting.UpdateDraft(context.Background(), user, draft.ID, MeetingBookingDraftUpdate{Title: &title, AttendeesConfirmed: &confirmed, Version: draft.Version}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("expected duplicate draft completion to conflict, got %v", err)
	}
}

func TestMeetingCancellationOrdinalDoesNotTreatTimeAsChoice(t *testing.T) {
	if got := ordinalChoice("改成16:30"); got != -1 {
		t.Fatalf("time must not be interpreted as a cancellation choice, got %d", got)
	}
	if got := ordinalChoice("选择第二个"); got != 1 {
		t.Fatalf("expected second choice, got %d", got)
	}
}

func TestPrepareCancellationOnlyAllowsOwnerAndCancelsCleanupDelivery(t *testing.T) {
	fake := &fakeMeetingFeishu{}
	meeting, repo, user, base := testMeetingService(t, fake)
	booking := domain.MeetingBooking{ID: "00000000-0000-4000-8000-000000000601", UserID: user.ID, CalendarID: "calendar", EventID: "event-cancel", RoomID: "room-small", RoomName: "海棠", Title: "产品周报评审", StartAt: base.Add(time.Hour), EndAt: base.Add(2 * time.Hour), Attendees: []domain.MeetingAttendee{{UserID: user.ID, OpenID: user.FeishuOpenID, Name: user.Name}}, Status: "active", SourceChannel: "h5", CreatedAt: base, UpdatedAt: base}
	if err := repo.CreateMeetingBooking(context.Background(), booking); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateMeetingBookingDelivery(context.Background(), domain.MeetingBookingDelivery{ID: "00000000-0000-4000-8000-000000000602", BookingID: booking.ID, ScheduledFor: booking.EndAt, Status: "pending", NextAttemptAt: booking.EndAt, IdempotencyKey: "cancel-delivery", CreatedAt: base, UpdatedAt: base}); err != nil {
		t.Fatal(err)
	}
	action, err := meeting.PrepareCancellationForBooking(context.Background(), user, booking.ID, "h5", "")
	if err != nil {
		t.Fatal(err)
	}
	_, cancelled, err := meeting.Confirm(context.Background(), user, action.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if cancelled == nil || cancelled.Status != "cancelled" || fake.deletedEvents != 1 {
		t.Fatalf("unexpected cancellation result: booking=%#v deleted=%d", cancelled, fake.deletedEvents)
	}
	other, _ := repo.GetUser(context.Background(), store.DemoAdminID)
	if _, err = meeting.PrepareCancellationForBooking(context.Background(), other, booking.ID, "h5", ""); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("expected owner check, got %v", err)
	}
}

func TestNaturalLanguageCancellationMatchesTitleWithoutDate(t *testing.T) {
	fake := &fakeMeetingFeishu{}
	meeting, repo, user, base := testMeetingService(t, fake)
	booking := domain.MeetingBooking{ID: "00000000-0000-4000-8000-000000000701", UserID: user.ID, CalendarID: "calendar", EventID: "event-title", RoomID: "room-small", RoomName: "海棠", Title: "周报评审", StartAt: base.AddDate(0, 0, 1), EndAt: base.AddDate(0, 0, 1).Add(time.Hour), Attendees: []domain.MeetingAttendee{{UserID: user.ID, OpenID: user.FeishuOpenID, Name: user.Name}}, Status: "active", SourceChannel: "h5", CreatedAt: base, UpdatedAt: base}
	if err := repo.CreateMeetingBooking(context.Background(), booking); err != nil {
		t.Fatal(err)
	}
	handled, message, err := meeting.HandleChat(context.Background(), user, "conversation-2", "取消周报评审会议", "h5")
	if err != nil || !handled || message.MeetingBookingAction == nil || message.MeetingBookingAction.BookingID != booking.ID {
		t.Fatalf("expected title cancellation action, handled=%v message=%#v err=%v", handled, message, err)
	}
}

func TestCancellationDraftCanSelectMeetingByPartialTitle(t *testing.T) {
	fake := &fakeMeetingFeishu{}
	meeting, repo, user, base := testMeetingService(t, fake)
	for index, title := range []string{"周报评审", "需求讨论"} {
		booking := domain.MeetingBooking{ID: fmt.Sprintf("00000000-0000-4000-8000-0000000008%02d", index), UserID: user.ID, CalendarID: "calendar", EventID: fmt.Sprintf("event-%d", index), RoomID: "room-small", RoomName: "海棠", Title: title, StartAt: base.AddDate(0, 0, 1).Add(time.Duration(index) * time.Hour), EndAt: base.AddDate(0, 0, 1).Add(time.Duration(index+1) * time.Hour), Attendees: []domain.MeetingAttendee{{UserID: user.ID, OpenID: user.FeishuOpenID, Name: user.Name}}, Status: "active", SourceChannel: "h5", CreatedAt: base, UpdatedAt: base}
		if err := repo.CreateMeetingBooking(context.Background(), booking); err != nil {
			t.Fatal(err)
		}
	}
	handled, message, err := meeting.HandleChat(context.Background(), user, "conversation-3", "取消明天的会议", "h5")
	if err != nil || !handled || message.MeetingBookingDraft == nil {
		t.Fatalf("expected cancellation selection draft, handled=%v message=%#v err=%v", handled, message, err)
	}
	handled, message, err = meeting.HandleChat(context.Background(), user, "conversation-3", "取消周报那个", "h5")
	if err != nil || !handled || message.MeetingBookingAction == nil || message.MeetingBookingAction.Title != "周报评审" {
		t.Fatalf("expected partial-title cancellation action, handled=%v message=%#v err=%v", handled, message, err)
	}
}
