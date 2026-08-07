package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/store"
)

type MeetingFeishuClient interface {
	Configured() bool
	ListMeetingRooms(context.Context) ([]feishu.MeetingRoomInfo, error)
	GetMeetingRoomReserveConfig(context.Context, string) (feishu.MeetingRoomReserveConfig, error)
	MeetingRoomFreeBusy(context.Context, []string, time.Time, time.Time) (map[string][]feishu.BusyInterval, error)
	UserFreeBusy(context.Context, []string, time.Time, time.Time) (map[string][]feishu.BusyInterval, error)
	CreateSharedCalendar(context.Context, string) (string, error)
	CreateCalendarEvent(context.Context, string, string, string, time.Time, time.Time, string) (feishu.CalendarEventInfo, error)
	AddCalendarEventAttendees(context.Context, string, string, []feishu.EventAttendeeInput, bool) ([]feishu.EventAttendee, error)
	ListCalendarEventAttendees(context.Context, string, string) ([]feishu.EventAttendee, error)
	DeleteCalendarEvent(context.Context, string, string, bool) error
	SendMeetingBookingResult(context.Context, string, string, string, string, string) (string, error)
	SendMeetingRoomCleanupReminder(context.Context, string, string, string) (string, error)
}

type Meeting struct {
	repo               store.Repository
	feishu             MeetingFeishuClient
	locker             SlotLocker
	location           *time.Location
	configuredCalendar string
	now                func() time.Time
}

var (
	errMeetingRoomsUnsynced = errors.New("会议室列表为空，请管理员先在会议室管理页同步")
	errMeetingTimeInPast    = errors.New("会议开始时间必须晚于当前时间")
)

func NewMeeting(repo store.Repository, client MeetingFeishuClient, locker SlotLocker, timezone, calendarID string) *Meeting {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*3600)
	}
	if locker == nil {
		locker = NewLocalSlotLocker()
	}
	return &Meeting{repo: repo, feishu: client, locker: locker, location: location, configuredCalendar: strings.TrimSpace(calendarID), now: time.Now}
}

func (m *Meeting) Configured() bool { return m != nil && m.feishu != nil && m.feishu.Configured() }

func (m *Meeting) settings(ctx context.Context) (domain.MeetingSettings, error) {
	value, err := m.repo.GetMeetingSettings(ctx)
	if errors.Is(err, store.ErrNotFound) {
		value = domain.MeetingSettings{CalendarID: m.configuredCalendar, Timezone: m.location.String(), WorkdayStart: "09:00", WorkdayEnd: "18:00", SlotMinutes: 30, SyncIntervalMinute: 15, UpdatedAt: m.now()}
		err = m.repo.SaveMeetingSettings(ctx, value)
	} else if err == nil && value.CalendarID == "" && m.configuredCalendar != "" {
		value.CalendarID = m.configuredCalendar
		value.UpdatedAt = m.now()
		err = m.repo.SaveMeetingSettings(ctx, value)
	}
	return value, err
}

func (m *Meeting) InitializeCalendar(ctx context.Context, actor domain.User) (domain.MeetingSettings, error) {
	settings, err := m.settings(ctx)
	if err != nil {
		return settings, err
	}
	if settings.CalendarID != "" {
		return settings, nil
	}
	if !m.Configured() {
		return settings, errors.New("飞书应用未配置")
	}
	calendarID, err := m.feishu.CreateSharedCalendar(ctx, "行政 AI 会议室预约")
	if err != nil {
		return settings, err
	}
	settings.CalendarID = calendarID
	settings.UpdatedAt = m.now()
	if err = m.repo.SaveMeetingSettings(ctx, settings); err != nil {
		return settings, err
	}
	m.audit(ctx, actor, "meeting.calendar.initialize", "meeting_settings", "shared_calendar", map[string]any{"calendar_id": calendarID})
	return settings, nil
}

func (m *Meeting) SyncRooms(ctx context.Context, actor *domain.User) ([]domain.MeetingRoom, error) {
	settings, err := m.settings(ctx)
	if err != nil {
		return nil, err
	}
	if !m.Configured() {
		return nil, errors.New("飞书应用未配置")
	}
	remoteRooms, err := m.feishu.ListMeetingRooms(ctx)
	if err != nil {
		settings.LastSyncError = err.Error()
		settings.UpdatedAt = m.now()
		_ = m.repo.SaveMeetingSettings(ctx, settings)
		return nil, err
	}
	now := m.now()
	rooms := make([]domain.MeetingRoom, 0, len(remoteRooms))
	for _, remote := range remoteRooms {
		room := domain.MeetingRoom{ID: ids.New("room"), RoomID: remote.RoomID, Name: remote.Name, Capacity: remote.Capacity, RoomLevelID: remote.RoomLevelID, Path: remote.Path, Enabled: remote.Enabled, ScheduleEnabled: remote.ScheduleEnabled, DisabledFrom: remote.DisableStart, DisabledUntil: remote.DisableEnd, DisableReason: remote.DisableReason, ReservationStartSeconds: 9 * 3600, ReservationEndSeconds: 18 * 3600, MaxDurationHours: 2, SyncedAt: now}
		reserve, reserveErr := m.feishu.GetMeetingRoomReserveConfig(ctx, remote.RoomID)
		if reserveErr != nil {
			room.LastError = "无法读取预定限制：" + reserveErr.Error()
		} else {
			room.ApprovalSwitch = reserve.ApprovalSwitch
			room.ApprovalCondition = reserve.ApprovalCondition
			room.ApprovalDurationHours = reserve.ApprovalDurationHours
			room.ReservationStartSeconds = reserve.StartSeconds
			room.ReservationEndSeconds = reserve.EndSeconds
			room.MaxDurationHours = reserve.MaxDurationHours
		}
		rooms = append(rooms, room)
	}
	if err = m.repo.UpsertMeetingRooms(ctx, rooms); err != nil {
		return nil, err
	}
	settings.LastSyncedAt = &now
	settings.LastSyncError = ""
	settings.UpdatedAt = now
	if err = m.repo.SaveMeetingSettings(ctx, settings); err != nil {
		return nil, err
	}
	if actor != nil {
		m.audit(ctx, *actor, "meeting.rooms.sync", "meeting_room", "all", map[string]any{"count": len(rooms)})
	}
	return m.repo.ListMeetingRooms(ctx)
}

func (m *Meeting) Rooms(ctx context.Context) ([]domain.MeetingRoom, domain.MeetingSettings, error) {
	settings, err := m.settings(ctx)
	if err != nil {
		return nil, settings, err
	}
	rooms, err := m.repo.ListMeetingRooms(ctx)
	return rooms, settings, err
}

func (m *Meeting) BookingAlerts(ctx context.Context) ([]domain.MeetingBooking, error) {
	bookings, err := m.repo.ListMeetingBookings(ctx, "")
	if err != nil {
		return nil, err
	}
	alerts := make([]domain.MeetingBooking, 0)
	for _, booking := range bookings {
		if booking.Status == "needs_admin" {
			alerts = append(alerts, booking)
		}
		if len(alerts) == 100 {
			break
		}
	}
	return alerts, nil
}

func (m *Meeting) HandleMeetingRoomStatusChange(ctx context.Context, event feishu.MeetingRoomStatusChangeEvent) error {
	if event.EventID != "" && !m.repo.MarkEventProcessed(ctx, event.EventID) {
		return nil
	}
	_, err := m.SyncRooms(ctx, nil)
	if err != nil && event.EventID != "" {
		m.repo.ForgetProcessedEvent(ctx, event.EventID)
	}
	return err
}

func (m *Meeting) HandleChat(ctx context.Context, user domain.User, conversationID, question, source string) (bool, domain.Message, error) {
	if handled, message, err := m.handleActiveDraftTurn(ctx, user, conversationID, question, source); handled || err != nil {
		return handled, message, err
	}
	if !LooksLikeMeetingBooking(question) {
		return false, domain.Message{}, nil
	}
	now := m.now().In(m.location)
	request, clarification := parseMeetingRequest(question, now, m.location)
	if clarification != "" {
		return true, m.message(conversationID, clarification, nil), nil
	}
	if request.Intent == "cancel" {
		message, prepareErr := m.prepareCancellation(ctx, user, conversationID, request, source)
		return true, message, prepareErr
	}
	if request.Intent == "reschedule" {
		message, prepareErr := m.prepareReschedule(ctx, user, conversationID, request, question, source)
		return true, message, prepareErr
	}
	if !m.Configured() {
		return true, m.message(conversationID, "会议室预约尚未完成飞书配置，请联系管理员。", nil), nil
	}
	if len(request.Times) > 0 && !meetingDateTime(request.Date, request.Times[0]).After(now) {
		return true, m.message(conversationID, meetingUserErrorMessage(errMeetingTimeInPast), nil), nil
	}
	request.Title = strings.TrimSpace(request.Title)
	if len([]rune(request.Title)) > 100 {
		return true, m.message(conversationID, "会议主题不能超过 100 个字符，请精简后重试。", nil), nil
	}
	attendeesConfirmed := meetingAttendeesExplicitlySelfOnly(question) || len(request.AttendeeNames) > 0
	attendeeIDs := []string{}
	attendees, clarification, err := m.resolveAttendees(ctx, user, request.AttendeeNames)
	if err != nil {
		return true, domain.Message{}, err
	}
	if clarification != "" {
		attendeesConfirmed = false
		attendees = []domain.MeetingAttendee{{UserID: user.ID, OpenID: user.FeishuOpenID, Name: user.Name}}
	}
	if len(attendees) > 50 {
		return true, m.message(conversationID, "每场会议最多包含 50 名内部参会人（含申请人），请减少参会人数。", nil), nil
	}
	for _, attendee := range attendees {
		if attendee.UserID != user.ID {
			attendeeIDs = append(attendeeIDs, attendee.UserID)
		}
	}
	if strings.TrimSpace(request.Title) == "" || !attendeesConfirmed {
		message, draftErr := m.startCreateDraft(ctx, user, conversationID, source, request, attendeeIDs, attendeesConfirmed)
		if draftErr == nil && clarification != "" {
			message.Content = clarification + " 请在人员选择器中选择准确的同事。"
		}
		return true, message, draftErr
	}
	options, requestedRoom, err := m.candidates(ctx, request, question, attendees)
	if err != nil {
		if message := meetingUserErrorMessage(err); message != "" {
			return true, m.message(conversationID, message, nil), nil
		}
		return true, domain.Message{}, err
	}
	if len(options) == 0 {
		return true, m.message(conversationID, "指定当天没有同时满足会议室容量、会议室空闲和参会人空闲的方案。请换一个日期或时间。", nil), nil
	}
	action := domain.MeetingBookingAction{ID: ids.New("mba"), UserID: user.ID, Intent: "create", Title: request.Title, Attendees: attendees, Capacity: maxInt(request.Capacity, len(attendees)), RequestedRoomName: requestedRoom, Options: options, Status: domain.MeetingActionPending, SourceChannel: source, SourceConversationID: conversationID, ExpiresAt: now.Add(15 * time.Minute), CreatedAt: now}
	if err = m.repo.CreateMeetingBookingAction(ctx, action); err != nil {
		return true, domain.Message{}, err
	}
	m.audit(ctx, user, "meeting.action.create", "meeting_booking_action", action.ID, map[string]any{"intent": action.Intent, "options": len(options)})
	content := "我找到了可用会议室。请选择候选方案并确认，确认时会再次校验会议室和所有参会人的忙闲。"
	return true, m.message(conversationID, content, &action), nil
}

func (m *Meeting) prepareCancellation(ctx context.Context, user domain.User, conversationID string, request meetingRequest, source string) (domain.Message, error) {
	booking, matches, clarification, err := m.matchBookingCandidates(ctx, user, request, false)
	if err != nil {
		return domain.Message{}, err
	}
	if clarification != "" {
		if len(matches) > 1 {
			return m.cancellationDraft(ctx, user, conversationID, source, matches)
		}
		return m.message(conversationID, clarification, nil), nil
	}
	action, err := m.PrepareCancellationForBooking(ctx, user, booking.ID, source, conversationID)
	if err != nil {
		return domain.Message{}, err
	}
	return m.message(conversationID, fmt.Sprintf("请确认取消 %s 在 %s 的预约。", booking.RoomName, meetingTimeLabel(booking.StartAt.In(m.location), booking.EndAt.In(m.location))), &action), nil
}

func (m *Meeting) prepareReschedule(ctx context.Context, user domain.User, conversationID string, request meetingRequest, question, source string) (domain.Message, error) {
	booking, _, clarification, err := m.matchBookingCandidates(ctx, user, request, true)
	if err != nil {
		return domain.Message{}, err
	}
	if clarification != "" {
		return m.message(conversationID, clarification, nil), nil
	}
	if len(request.Times) == 0 {
		return m.message(conversationID, "请告诉我要改到当天几点。", nil), nil
	}
	request.Times = []clockTime{request.Times[len(request.Times)-1]}
	request.Duration = booking.EndAt.Sub(booking.StartAt)
	request.Title = booking.Title
	options, requestedRoom, err := m.candidates(ctx, request, question, booking.Attendees)
	if err != nil {
		if message := meetingUserErrorMessage(err); message != "" {
			return m.message(conversationID, message+" 原预约保持不变。", nil), nil
		}
		return domain.Message{}, err
	}
	if len(options) == 0 {
		return m.message(conversationID, "新时间没有可用方案，原预约保持不变。请换一个日期或时间。", nil), nil
	}
	now := m.now()
	action := domain.MeetingBookingAction{ID: ids.New("mba"), UserID: user.ID, Intent: "reschedule", Title: booking.Title, Attendees: booking.Attendees, Capacity: len(booking.Attendees), RequestedRoomName: requestedRoom, Options: options, BookingID: booking.ID, Status: domain.MeetingActionPending, SourceChannel: source, SourceConversationID: conversationID, ExpiresAt: now.Add(15 * time.Minute), CreatedAt: now}
	if err = m.repo.CreateMeetingBookingAction(ctx, action); err != nil {
		return domain.Message{}, err
	}
	return m.message(conversationID, "已保留原预约。请选择新方案并确认；只有新会议室预约成功后才会释放原会议室。", &action), nil
}

func (m *Meeting) resolveAttendees(ctx context.Context, requester domain.User, names []string) ([]domain.MeetingAttendee, string, error) {
	attendees := []domain.MeetingAttendee{{UserID: requester.ID, OpenID: requester.FeishuOpenID, Name: requester.Name}}
	users, err := m.repo.ListUsers(ctx)
	if err != nil {
		return nil, "", err
	}
	for _, name := range names {
		matches := []domain.User{}
		for _, user := range users {
			if user.Status == "active" && strings.EqualFold(strings.TrimSpace(user.Name), strings.TrimSpace(name)) {
				matches = append(matches, user)
			}
		}
		if len(matches) == 0 {
			return nil, fmt.Sprintf("通讯录中没有找到“%s”。请确认姓名，或让管理员先同步完整通讯录。", name), nil
		}
		if len(matches) > 1 {
			return nil, fmt.Sprintf("通讯录中有多位“%s”，暂时无法安全判断。请联系管理员补充唯一标识后再预约。", name), nil
		}
		match := matches[0]
		found := false
		for _, attendee := range attendees {
			found = found || attendee.UserID == match.ID
		}
		if !found {
			attendees = append(attendees, domain.MeetingAttendee{UserID: match.ID, OpenID: match.FeishuOpenID, Name: match.Name})
		}
	}
	for _, attendee := range attendees {
		if attendee.OpenID == "" {
			return nil, fmt.Sprintf("参会人“%s”缺少飞书 open_id，无法校验忙闲。", attendee.Name), nil
		}
	}
	return attendees, "", nil
}

func (m *Meeting) candidates(ctx context.Context, request meetingRequest, question string, attendees []domain.MeetingAttendee) ([]domain.MeetingBookingOption, string, error) {
	settings, err := m.settings(ctx)
	if err != nil {
		return nil, "", err
	}
	rooms, err := m.repo.ListMeetingRooms(ctx)
	if err != nil {
		return nil, "", err
	}
	if len(rooms) == 0 {
		return nil, "", errMeetingRoomsUnsynced
	}
	requestedRoom := requestedMeetingRoomName(question, rooms)
	start := meetingDateTime(request.Date, request.Times[len(request.Times)-1])
	if !start.After(m.now()) {
		return nil, requestedRoom, errMeetingTimeInPast
	}
	workStart, err := clockOnDate(request.Date, settings.WorkdayStart)
	if err != nil {
		return nil, requestedRoom, err
	}
	workEnd, err := clockOnDate(request.Date, settings.WorkdayEnd)
	if err != nil {
		return nil, requestedRoom, err
	}
	if start.Before(workStart) {
		start = workStart
	}
	if !start.Add(request.Duration).Before(workEnd) && !start.Add(request.Duration).Equal(workEnd) {
		return nil, requestedRoom, nil
	}
	capacity := maxInt(request.Capacity, len(attendees))
	eligible := []domain.MeetingRoom{}
	for _, room := range rooms {
		if room.LastError != "" || room.Capacity < capacity || requestedRoom != "" && room.Name != requestedRoom || room.MaxDurationHours > 0 && request.Duration > time.Duration(room.MaxDurationHours)*time.Hour || room.RequiresApproval(request.Duration) {
			continue
		}
		eligible = append(eligible, room)
	}
	if len(eligible) == 0 {
		return nil, requestedRoom, nil
	}
	openIDs := make([]string, 0, len(attendees))
	for _, attendee := range attendees {
		openIDs = append(openIDs, attendee.OpenID)
	}
	userBusy := map[string][]feishu.BusyInterval{}
	for offset := 0; offset < len(openIDs); offset += 10 {
		limit := offset + 10
		if limit > len(openIDs) {
			limit = len(openIDs)
		}
		batch, batchErr := m.feishu.UserFreeBusy(ctx, openIDs[offset:limit], start, workEnd)
		if batchErr != nil {
			return nil, requestedRoom, batchErr
		}
		for key, intervals := range batch {
			userBusy[key] = append(userBusy[key], intervals...)
		}
	}
	roomBusy := map[string][]feishu.BusyInterval{}
	roomIDs := make([]string, 0, len(eligible))
	for _, room := range eligible {
		roomIDs = append(roomIDs, room.RoomID)
	}
	for offset := 0; offset < len(roomIDs); offset += 20 {
		limit := offset + 20
		if limit > len(roomIDs) {
			limit = len(roomIDs)
		}
		batch, batchErr := m.feishu.MeetingRoomFreeBusy(ctx, roomIDs[offset:limit], start, workEnd)
		if batchErr != nil {
			return nil, requestedRoom, batchErr
		}
		for key, intervals := range batch {
			roomBusy[key] = append(roomBusy[key], intervals...)
		}
	}
	step := time.Duration(settings.SlotMinutes) * time.Minute
	if step <= 0 {
		step = 30 * time.Minute
	}
	type ranked struct {
		option domain.MeetingBookingOption
		delta  time.Duration
	}
	values := []ranked{}
	requestedStart := start
	for candidateStart := start; !candidateStart.Add(request.Duration).After(workEnd); candidateStart = candidateStart.Add(step) {
		candidateEnd := candidateStart.Add(request.Duration)
		if usersConflict(openIDs, userBusy, candidateStart, candidateEnd) {
			continue
		}
		seconds := candidateStart.Hour()*3600 + candidateStart.Minute()*60
		endSeconds := candidateEnd.Hour()*3600 + candidateEnd.Minute()*60
		for _, room := range eligible {
			if !room.AvailableAt(candidateStart, candidateEnd) || seconds < maxInt(room.ReservationStartSeconds, 9*3600) || endSeconds > minPositive(room.ReservationEndSeconds, 18*3600) || overlapsBusy(roomBusy[room.RoomID], candidateStart, candidateEnd, false) {
				continue
			}
			values = append(values, ranked{option: domain.MeetingBookingOption{ID: ids.New("mbo"), RoomID: room.RoomID, RoomName: room.Name, Capacity: room.Capacity, StartAt: candidateStart, EndAt: candidateEnd}, delta: candidateStart.Sub(requestedStart)})
		}
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].delta != values[j].delta {
			return values[i].delta < values[j].delta
		}
		leftGap, rightGap := values[i].option.Capacity-capacity, values[j].option.Capacity-capacity
		if leftGap != rightGap {
			return leftGap < rightGap
		}
		return values[i].option.RoomName < values[j].option.RoomName
	})
	options := []domain.MeetingBookingOption{}
	for _, value := range values {
		duplicateTime := false
		for _, option := range options {
			duplicateTime = duplicateTime || option.RoomID == value.option.RoomID && option.StartAt.Equal(value.option.StartAt)
		}
		if !duplicateTime {
			options = append(options, value.option)
		}
		if len(options) == 3 {
			break
		}
	}
	return options, requestedRoom, nil
}

func requestedMeetingRoomName(text string, rooms []domain.MeetingRoom) string {
	for _, room := range rooms {
		if room.Name != "" && strings.Contains(text, room.Name) {
			return room.Name
		}
	}
	match := meetingRoomNo.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	matches := make([]string, 0, 1)
	for _, room := range rooms {
		roomMatch := meetingRoomNo.FindStringSubmatch(room.Name)
		if len(roomMatch) > 1 && roomMatch[1] == match[1] {
			matches = append(matches, room.Name)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

func (m *Meeting) Confirm(ctx context.Context, user domain.User, actionID, optionID string) (domain.MeetingBookingAction, *domain.MeetingBooking, error) {
	action, err := m.repo.GetMeetingBookingAction(ctx, actionID)
	if err != nil {
		return action, nil, err
	}
	if action.UserID != user.ID {
		return action, nil, store.ErrForbidden
	}
	if optionID == "" && len(action.Options) > 0 {
		optionID = action.Options[0].ID
	}
	action, err = m.repo.ClaimMeetingBookingAction(ctx, actionID, user.ID, optionID, m.now())
	if err != nil {
		if errors.Is(err, store.ErrConflict) && action.Status == domain.MeetingActionConfirmed && action.ResultBookingID != "" {
			booking, getErr := m.repo.GetMeetingBooking(ctx, action.ResultBookingID)
			return action, &booking, getErr
		}
		return action, nil, err
	}
	if action.Intent == "cancel" {
		return m.confirmCancellation(ctx, user, action)
	}
	option, found := findMeetingOption(action.Options, optionID)
	if !found {
		m.releaseAction(ctx, &action)
		return action, nil, errors.New("候选会议室不存在")
	}
	attendeeIDs := make([]string, 0, len(action.Attendees))
	for _, attendee := range action.Attendees {
		if attendee.UserID != user.ID {
			attendeeIDs = append(attendeeIDs, attendee.UserID)
		}
	}
	if action.Attendees, err = m.resolveAttendeeIDs(ctx, user, attendeeIDs); err != nil {
		m.releaseAction(ctx, &action)
		return action, nil, err
	}
	unlock, locked, err := m.locker.Lock(ctx, fmt.Sprintf("%s:%d:%d", option.RoomID, option.StartAt.Unix(), option.EndAt.Unix()), 3*time.Minute)
	if err != nil || !locked {
		m.releaseAction(ctx, &action)
		if err != nil {
			return action, nil, err
		}
		return action, nil, store.ErrConflict
	}
	defer unlock()
	if err = m.ensureOptionAvailable(ctx, action.Attendees, option); err != nil {
		m.releaseAction(ctx, &action)
		return action, nil, fmt.Errorf("确认时会议室或参会人已被占用，请重新生成候选方案: %w", err)
	}
	settings, err := m.settings(ctx)
	if err != nil || settings.CalendarID == "" {
		m.releaseAction(ctx, &action)
		if err == nil {
			err = errors.New("尚未初始化行政 AI 共享日历")
		}
		return action, nil, err
	}
	event, err := m.feishu.CreateCalendarEvent(ctx, settings.CalendarID, action.Title, "由行政 AI 会议室预约创建。取消或改期请通过行政 AI。", option.StartAt, option.EndAt, "meeting-event-"+action.ID)
	if err != nil {
		m.releaseAction(ctx, &action)
		return action, nil, err
	}
	roomAccepted, roomErr := m.reserveRoom(ctx, settings.CalendarID, event.EventID, option.RoomID, user.FeishuOpenID)
	if roomErr != nil || !roomAccepted {
		cleanupErr := m.feishu.DeleteCalendarEvent(ctx, settings.CalendarID, event.EventID, false)
		m.releaseAction(ctx, &action)
		if cleanupErr != nil {
			m.recordNeedsAdmin(ctx, user, action, option, settings.CalendarID, event.EventID, cleanupErr)
			return action, nil, fmt.Errorf("会议室预约失败且临时日程清理失败，已通知管理员: %w", cleanupErr)
		}
		if roomErr != nil {
			return action, nil, roomErr
		}
		return action, nil, errors.New("会议室拒绝了预约")
	}
	userInputs := make([]feishu.EventAttendeeInput, 0, len(action.Attendees))
	for _, attendee := range action.Attendees {
		userInputs = append(userInputs, feishu.EventAttendeeInput{Type: "user", UserID: attendee.OpenID})
	}
	if _, err = m.feishu.AddCalendarEventAttendees(ctx, settings.CalendarID, event.EventID, userInputs, true); err != nil {
		cleanupErr := m.feishu.DeleteCalendarEvent(ctx, settings.CalendarID, event.EventID, false)
		m.releaseAction(ctx, &action)
		if cleanupErr != nil {
			m.recordNeedsAdmin(ctx, user, action, option, settings.CalendarID, event.EventID, cleanupErr)
		}
		return action, nil, err
	}
	now := m.now()
	booking := domain.MeetingBooking{ID: ids.New("mbk"), UserID: user.ID, CalendarID: settings.CalendarID, EventID: event.EventID, RoomID: option.RoomID, RoomName: option.RoomName, Title: action.Title, StartAt: option.StartAt, EndAt: option.EndAt, Attendees: action.Attendees, Status: "active", SourceChannel: action.SourceChannel, SourceConversationID: action.SourceConversationID, CreatedAt: now, UpdatedAt: now}
	if action.Intent == "reschedule" {
		booking.ReplacesBookingID = action.BookingID
	}
	if err = m.repo.CreateMeetingBooking(ctx, booking); err != nil {
		cleanupErr := m.feishu.DeleteCalendarEvent(ctx, settings.CalendarID, event.EventID, false)
		m.releaseAction(ctx, &action)
		if cleanupErr != nil {
			m.recordNeedsAdmin(ctx, user, action, option, settings.CalendarID, event.EventID, cleanupErr)
		}
		return action, nil, err
	}
	delivery := domain.MeetingBookingDelivery{ID: ids.New("mbd"), BookingID: booking.ID, ScheduledFor: booking.EndAt, Status: "pending", NextAttemptAt: booking.EndAt, IdempotencyKey: meetingCleanupIdempotencyKey(booking.ID), CreatedAt: now, UpdatedAt: now}
	if deliveryErr := m.repo.CreateMeetingBookingDelivery(ctx, delivery); deliveryErr != nil {
		slog.Error("create meeting cleanup reminder delivery failed", "booking_id", booking.ID, "error", deliveryErr)
		m.audit(ctx, user, "meeting.delivery.create_failed", "meeting_booking", booking.ID, map[string]any{"error": deliveryErr.Error()})
	}
	if action.Intent == "reschedule" {
		oldBooking, oldErr := m.repo.GetMeetingBooking(ctx, action.BookingID)
		if oldErr == nil {
			if deleteErr := m.feishu.DeleteCalendarEvent(ctx, oldBooking.CalendarID, oldBooking.EventID, true); deleteErr != nil {
				oldBooking.Status, oldBooking.LastError, oldBooking.ReplacedByBookingID, oldBooking.UpdatedAt = "needs_admin", deleteErr.Error(), booking.ID, now
				booking.Status, booking.LastError, booking.UpdatedAt = "needs_admin", "新预约成功但原日程释放失败", now
				_ = m.repo.UpdateMeetingBooking(ctx, oldBooking)
				_ = m.repo.UpdateMeetingBooking(ctx, booking)
			} else {
				oldBooking.Status, oldBooking.ReplacedByBookingID, oldBooking.UpdatedAt = "replaced", booking.ID, now
				_ = m.repo.UpdateMeetingBooking(ctx, oldBooking)
			}
		}
	}
	confirmed := now
	action.Status, action.ResultBookingID, action.ConfirmedAt = domain.MeetingActionConfirmed, booking.ID, &confirmed
	if err = m.repo.UpdateMeetingBookingAction(ctx, action); err != nil {
		return action, &booking, err
	}
	m.audit(ctx, user, "meeting.booking.confirm", "meeting_booking", booking.ID, map[string]any{"room_id": booking.RoomID, "event_id": booking.EventID, "intent": action.Intent})
	if user.FeishuOpenID != "" {
		content := fmt.Sprintf("**%s**\n%s\n会议室：%s\n参会人：%s", booking.Title, meetingTimeLabel(booking.StartAt.In(m.location), booking.EndAt.In(m.location)), booking.RoomName, attendeeNames(booking.Attendees))
		_, _ = m.feishu.SendMeetingBookingResult(ctx, user.FeishuOpenID, "会议室预约成功", content, "green", "meeting-success-"+booking.ID)
	}
	return action, &booking, nil
}

func (m *Meeting) confirmCancellation(ctx context.Context, user domain.User, action domain.MeetingBookingAction) (domain.MeetingBookingAction, *domain.MeetingBooking, error) {
	booking, err := m.repo.GetMeetingBooking(ctx, action.BookingID)
	if err != nil {
		m.releaseAction(ctx, &action)
		return action, nil, err
	}
	if booking.UserID != user.ID {
		m.releaseAction(ctx, &action)
		return action, nil, store.ErrForbidden
	}
	if err = m.feishu.DeleteCalendarEvent(ctx, booking.CalendarID, booking.EventID, true); err != nil {
		m.releaseAction(ctx, &action)
		return action, nil, err
	}
	now := m.now()
	booking.Status, booking.UpdatedAt = "cancelled", now
	if err = m.repo.UpdateMeetingBooking(ctx, booking); err != nil {
		return action, &booking, err
	}
	action.Status, action.ResultBookingID, action.ConfirmedAt = domain.MeetingActionConfirmed, booking.ID, &now
	if err = m.repo.UpdateMeetingBookingAction(ctx, action); err != nil {
		return action, &booking, err
	}
	m.audit(ctx, user, "meeting.booking.cancel", "meeting_booking", booking.ID, map[string]any{"event_id": booking.EventID})
	m.audit(ctx, user, "meeting.booking.cancel_notification", "meeting_booking", booking.ID, map[string]any{"channel": "feishu_calendar", "attendee_count": len(booking.Attendees), "need_notification": true})
	return action, &booking, nil
}

func (m *Meeting) CancelAction(ctx context.Context, user domain.User, actionID string) (domain.MeetingBookingAction, error) {
	action, err := m.repo.GetMeetingBookingAction(ctx, actionID)
	if err != nil {
		return action, err
	}
	if action.UserID != user.ID {
		return action, store.ErrForbidden
	}
	if action.Status == domain.MeetingActionCancelled {
		return action, nil
	}
	if action.Status != domain.MeetingActionPending {
		return action, store.ErrConflict
	}
	now := m.now()
	action.Status, action.ConfirmedAt = domain.MeetingActionCancelled, &now
	err = m.repo.UpdateMeetingBookingAction(ctx, action)
	return action, err
}

func (m *Meeting) ListBookings(ctx context.Context, user domain.User) ([]domain.MeetingBooking, error) {
	return m.repo.ListMeetingBookings(ctx, user.ID)
}

func (m *Meeting) ensureOptionAvailable(ctx context.Context, attendees []domain.MeetingAttendee, option domain.MeetingBookingOption) error {
	room, err := m.repo.GetMeetingRoom(ctx, option.RoomID)
	if err != nil {
		return err
	}
	if room.LastError != "" || room.RequiresApproval(option.EndAt.Sub(option.StartAt)) || !room.AvailableAt(option.StartAt, option.EndAt) {
		return errors.New("会议室当前不可自动预约")
	}
	roomBusy, err := m.feishu.MeetingRoomFreeBusy(ctx, []string{option.RoomID}, option.StartAt, option.EndAt)
	if err != nil {
		return err
	}
	if overlapsBusy(roomBusy[option.RoomID], option.StartAt, option.EndAt, false) {
		return errors.New("会议室已占用")
	}
	openIDs := make([]string, 0, len(attendees))
	for _, attendee := range attendees {
		openIDs = append(openIDs, attendee.OpenID)
	}
	for offset := 0; offset < len(openIDs); offset += 10 {
		limit := offset + 10
		if limit > len(openIDs) {
			limit = len(openIDs)
		}
		busy, busyErr := m.feishu.UserFreeBusy(ctx, openIDs[offset:limit], option.StartAt, option.EndAt)
		if busyErr != nil {
			return busyErr
		}
		if usersConflict(openIDs[offset:limit], busy, option.StartAt, option.EndAt) {
			return errors.New("参会人已占用")
		}
	}
	return nil
}

func (m *Meeting) reserveRoom(ctx context.Context, calendarID, eventID, roomID, operatorOpenID string) (bool, error) {
	attendees, err := m.feishu.AddCalendarEventAttendees(ctx, calendarID, eventID, []feishu.EventAttendeeInput{{Type: "resource", RoomID: roomID, OperateID: operatorOpenID}}, false)
	if err != nil {
		return false, err
	}
	if status := roomRSVP(attendees, roomID); status == "accept" {
		return true, nil
	} else if status == "decline" {
		return false, nil
	}
	deadline := time.NewTimer(2 * time.Minute)
	defer deadline.Stop()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline.C:
			return false, errors.New("会议室 RSVP 在 2 分钟内未确认")
		case <-ticker.C:
			attendees, err = m.feishu.ListCalendarEventAttendees(ctx, calendarID, eventID)
			if err != nil {
				return false, err
			}
			switch roomRSVP(attendees, roomID) {
			case "accept":
				return true, nil
			case "decline":
				return false, nil
			}
		}
	}
}

func (m *Meeting) matchBookingCandidates(ctx context.Context, user domain.User, request meetingRequest, forReschedule bool) (domain.MeetingBooking, []domain.MeetingBooking, string, error) {
	bookings, err := m.repo.ListMeetingBookings(ctx, user.ID)
	if err != nil {
		return domain.MeetingBooking{}, nil, "", err
	}
	matches := []domain.MeetingBooking{}
	var target *clockTime
	if len(request.Times) > 0 {
		index := 0
		if !forReschedule || len(request.Times) == 1 {
			index = 0
		}
		target = &request.Times[index]
	}
	for _, booking := range bookings {
		start := booking.StartAt.In(m.location)
		if booking.Status != "active" || !booking.EndAt.After(m.now()) {
			continue
		}
		if !request.Date.IsZero() && (start.Year() != request.Date.Year() || start.YearDay() != request.Date.YearDay()) {
			continue
		}
		if target != nil && (start.Hour() != target.Hour || start.Minute() != target.Minute) {
			continue
		}
		if request.Title != "" && !strings.Contains(strings.ToLower(booking.Title), strings.ToLower(request.Title)) && !strings.Contains(strings.ToLower(request.Query), strings.ToLower(booking.Title)) {
			continue
		}
		if queryRoom := meetingRoomNo.FindStringSubmatch(request.Query); len(queryRoom) > 1 {
			bookingRoom := meetingRoomNo.FindStringSubmatch(booking.RoomName)
			if len(bookingRoom) < 2 || bookingRoom[1] != queryRoom[1] {
				continue
			}
		}
		matches = append(matches, booking)
	}
	if len(matches) == 0 {
		return domain.MeetingBooking{}, nil, "没有找到你尚未结束且由行政 AI 创建的匹配预约。", nil
	}
	if len(matches) > 1 {
		labels := make([]string, 0, len(matches))
		for _, booking := range matches {
			labels = append(labels, fmt.Sprintf("%s %s", booking.StartAt.In(m.location).Format("15:04"), booking.RoomName))
		}
		return domain.MeetingBooking{}, matches, "找到多个匹配预约，请选择：" + strings.Join(labels, "、"), nil
	}
	return matches[0], matches, "", nil
}

func (m *Meeting) message(conversationID, content string, action *domain.MeetingBookingAction) domain.Message {
	return domain.Message{ID: ids.New("msg"), ConversationID: conversationID, Role: "assistant", Content: content, Citations: []domain.Citation{}, MeetingBookingAction: action, CreatedAt: m.now()}
}

func (m *Meeting) releaseAction(ctx context.Context, action *domain.MeetingBookingAction) {
	action.Status = domain.MeetingActionPending
	_ = m.repo.UpdateMeetingBookingAction(ctx, *action)
}

func (m *Meeting) recordNeedsAdmin(ctx context.Context, user domain.User, action domain.MeetingBookingAction, option domain.MeetingBookingOption, calendarID, eventID string, cause error) {
	now := m.now()
	booking := domain.MeetingBooking{ID: ids.New("mbk"), UserID: user.ID, CalendarID: calendarID, EventID: eventID, RoomID: option.RoomID, RoomName: option.RoomName, Title: action.Title, StartAt: option.StartAt, EndAt: option.EndAt, Attendees: action.Attendees, Status: "needs_admin", LastError: cause.Error(), SourceChannel: action.SourceChannel, SourceConversationID: action.SourceConversationID, CreatedAt: now, UpdatedAt: now}
	_ = m.repo.CreateMeetingBooking(ctx, booking)
	// A failed compensation may have left a real event occupying the room. Mark the
	// action terminal so a second click cannot create another event for the same request.
	action.Status, action.ResultBookingID, action.ConfirmedAt = domain.MeetingActionExpired, booking.ID, &now
	_ = m.repo.UpdateMeetingBookingAction(ctx, action)
	m.audit(ctx, user, "meeting.cleanup.failed", "meeting_booking", booking.ID, map[string]any{"event_id": eventID, "error": cause.Error()})
}

func meetingUserErrorMessage(err error) string {
	switch {
	case errors.Is(err, errMeetingTimeInPast):
		return "会议开始时间必须晚于当前时间，请重新指定今天稍晚的时间或其他日期。"
	case errors.Is(err, errMeetingRoomsUnsynced):
		return "会议室列表尚未同步，请联系管理员在会议室管理页执行同步。"
	default:
		return ""
	}
}

func (m *Meeting) audit(ctx context.Context, actor domain.User, action, resourceType, resourceID string, metadata map[string]any) {
	actorName := actor.Name
	if actorName == "" {
		actorName = "系统"
	}
	_ = m.repo.AppendAudit(ctx, domain.AuditEvent{ID: ids.New("aud"), ActorID: actor.ID, ActorName: actorName, Action: action, ResourceType: resourceType, ResourceID: resourceID, Metadata: metadata, CreatedAt: m.now()})
}

func findMeetingOption(options []domain.MeetingBookingOption, id string) (domain.MeetingBookingOption, bool) {
	for _, option := range options {
		if option.ID == id {
			return option, true
		}
	}
	return domain.MeetingBookingOption{}, false
}

func roomRSVP(attendees []feishu.EventAttendee, roomID string) string {
	for _, attendee := range attendees {
		if attendee.Type == "resource" && attendee.RoomID == roomID {
			return strings.ToLower(attendee.RSVPStatus)
		}
	}
	return "needs_action"
}

func usersConflict(openIDs []string, busy map[string][]feishu.BusyInterval, start, end time.Time) bool {
	for _, openID := range openIDs {
		if overlapsBusy(busy[openID], start, end, true) {
			return true
		}
	}
	return false
}

func overlapsBusy(intervals []feishu.BusyInterval, start, end time.Time, honorRSVP bool) bool {
	for _, interval := range intervals {
		status := strings.ToLower(interval.RSVPStatus)
		if honorRSVP && (status == "decline" || status == "declined" || status == "removed") {
			continue
		}
		if interval.Start.Before(end) && start.Before(interval.End) {
			return true
		}
	}
	return false
}

func clockOnDate(date time.Time, value string) (time.Time, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid meeting workday time %q: %w", value, err)
	}
	return time.Date(date.Year(), date.Month(), date.Day(), parsed.Hour(), parsed.Minute(), 0, 0, date.Location()), nil
}

func minPositive(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if fallback <= 0 || value < fallback {
		return value
	}
	return fallback
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func attendeeNames(attendees []domain.MeetingAttendee) string {
	values := make([]string, 0, len(attendees))
	for _, attendee := range attendees {
		values = append(values, attendee.Name)
	}
	return strings.Join(values, "、")
}

func meetingCleanupIdempotencyKey(bookingID string) string {
	compactID := strings.ReplaceAll(strings.TrimSpace(bookingID), "-", "")
	const prefix = "mtg-clean-"
	maxIDLength := 50 - len(prefix)
	if len(compactID) > maxIDLength {
		compactID = compactID[:maxIDLength]
	}
	return prefix + compactID
}

type MeetingDispatcher struct {
	meeting         *Meeting
	repo            store.Repository
	lastSyncAttempt time.Time
}

func NewMeetingDispatcher(repo store.Repository, meeting *Meeting) *MeetingDispatcher {
	return &MeetingDispatcher{meeting: meeting, repo: repo}
}

func (d *MeetingDispatcher) Tick(ctx context.Context) error {
	if d == nil || d.meeting == nil || !d.meeting.Configured() {
		return nil
	}
	settings, err := d.meeting.settings(ctx)
	if err != nil {
		return err
	}
	interval := time.Duration(settings.SyncIntervalMinute) * time.Minute
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	now := d.meeting.now()
	lastSync := settings.LastSyncedAt
	dueFromLastSuccess := lastSync == nil || now.Sub(*lastSync) >= interval
	dueFromLastAttempt := d.lastSyncAttempt.IsZero() || now.Sub(d.lastSyncAttempt) >= interval
	if dueFromLastSuccess && dueFromLastAttempt {
		d.lastSyncAttempt = now
		if _, syncErr := d.meeting.SyncRooms(ctx, nil); syncErr != nil {
			slog.Error("meeting room sync failed", "error", syncErr)
		}
	}
	deliveries, err := d.repo.ClaimMeetingBookingDeliveries(ctx, now, time.Minute, 50)
	if err != nil {
		return err
	}
	for _, delivery := range deliveries {
		d.deliver(ctx, delivery)
	}
	return nil
}

func (d *MeetingDispatcher) deliver(ctx context.Context, delivery domain.MeetingBookingDelivery) {
	now := d.meeting.now()
	booking, err := d.repo.GetMeetingBooking(ctx, delivery.BookingID)
	if err == nil && booking.Status != "active" {
		delivery.Status, delivery.LockedUntil, delivery.UpdatedAt = "cancelled", nil, now
		_ = d.repo.UpdateMeetingBookingDelivery(ctx, delivery)
		return
	}
	var user domain.User
	if err == nil {
		user, err = d.repo.GetUser(ctx, booking.UserID)
	}
	if err == nil {
		content := fmt.Sprintf("你在 **%s** 的会议已结束，请：\n- 关闭显示器、视频会议等设备\n- 带走个人物品和垃圾\n- 恢复桌椅、白板及会议室环境", booking.RoomName)
		delivery.MessageID, err = d.meeting.feishu.SendMeetingRoomCleanupReminder(ctx, user.FeishuOpenID, content, delivery.IdempotencyKey)
	}
	if err == nil {
		delivery.Status, delivery.Error, delivery.LockedUntil, delivery.UpdatedAt = "sent", "", nil, now
		slog.Info("meeting cleanup reminder sent", "booking_id", booking.ID, "delivery_id", delivery.ID, "message_id", delivery.MessageID)
	} else if delivery.Attempts >= 5 {
		delivery.Status, delivery.Error, delivery.LockedUntil, delivery.UpdatedAt = "failed", err.Error(), nil, now
		slog.Error("meeting cleanup reminder permanently failed", "booking_id", delivery.BookingID, "delivery_id", delivery.ID, "attempts", delivery.Attempts, "error", err)
	} else {
		delivery.Status, delivery.Error, delivery.LockedUntil, delivery.NextAttemptAt, delivery.UpdatedAt = "retry", err.Error(), nil, now.Add(time.Duration(delivery.Attempts*delivery.Attempts)*time.Minute), now
		slog.Warn("meeting cleanup reminder will retry", "booking_id", delivery.BookingID, "delivery_id", delivery.ID, "attempts", delivery.Attempts, "next_attempt_at", delivery.NextAttemptAt, "error", err)
	}
	if updateErr := d.repo.UpdateMeetingBookingDelivery(ctx, delivery); updateErr != nil {
		slog.Error("update meeting cleanup reminder delivery failed", "booking_id", delivery.BookingID, "delivery_id", delivery.ID, "error", updateErr)
	}
}
