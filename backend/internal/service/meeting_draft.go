package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/store"
)

const (
	meetingDraftCollecting = "collecting"
	meetingDraftCompleted  = "completed"
	meetingDraftExpired    = "expired"
)

type MeetingBookingDraftUpdate struct {
	Title              *string  `json:"title,omitempty"`
	AttendeeUserIDs    []string `json:"attendee_user_ids"`
	AttendeesConfirmed *bool    `json:"attendees_confirmed,omitempty"`
	BookingID          string   `json:"booking_id,omitempty"`
	Version            int64    `json:"version"`
}

type MeetingBookingDraftResult struct {
	Draft  domain.MeetingBookingDraft   `json:"draft"`
	Action *domain.MeetingBookingAction `json:"action,omitempty"`
}

func (m *Meeting) startCreateDraft(ctx context.Context, user domain.User, conversationID, source string, request meetingRequest, attendeeIDs []string, attendeesConfirmed bool) (domain.Message, error) {
	now := m.now()
	slots := domain.MeetingBookingDraftSlots{
		Date: request.Date.In(m.location).Format("2006-01-02"), Hour: request.Times[0].Hour, Minute: request.Times[0].Minute,
		DurationMinutes: int(request.Duration / time.Minute), Title: strings.TrimSpace(request.Title), AttendeeUserIDs: attendeeIDs,
		AttendeesConfirmed: attendeesConfirmed, Capacity: request.Capacity, OriginalQuery: request.Query,
	}
	draft := domain.MeetingBookingDraft{ID: ids.New("mbd"), UserID: user.ID, ConversationID: conversationID, Channel: source, Intent: "create", Slots: slots, Status: meetingDraftCollecting, Version: 1, ExpiresAt: now.Add(15 * time.Minute), CreatedAt: now, UpdatedAt: now}
	draft.MissingFields = meetingDraftMissingFields(draft)
	if err := m.repo.CreateMeetingBookingDraft(ctx, draft); err != nil {
		return domain.Message{}, err
	}
	draft = m.enrichDraft(ctx, draft)
	m.audit(ctx, user, "meeting.draft.create", "meeting_booking_draft", draft.ID, map[string]any{"missing_fields": draft.MissingFields})
	return m.draftMessage(conversationID, meetingDraftPrompt(draft), &draft), nil
}

func meetingDraftMissingFields(draft domain.MeetingBookingDraft) []string {
	missing := []string{}
	if draft.Intent == "cancel" {
		if draft.Slots.SelectedBookingID == "" {
			missing = append(missing, "booking")
		}
		return missing
	}
	if strings.TrimSpace(draft.Slots.Title) == "" {
		missing = append(missing, "title")
	}
	if !draft.Slots.AttendeesConfirmed {
		missing = append(missing, "attendees")
	}
	return missing
}

func meetingDraftPrompt(draft domain.MeetingBookingDraft) string {
	if draft.Intent == "cancel" {
		return "找到了多个尚未结束的预约，请选择要取消的会议。"
	}
	if len(draft.MissingFields) == 2 {
		return "预约前请补充会议主题，并选择要邀请的飞书同事；如果没有其他参会人，可以选择“仅自己参会”。"
	}
	if len(draft.MissingFields) == 1 && draft.MissingFields[0] == "title" {
		return "请补充本次会议的主题。"
	}
	return "请选择要邀请的飞书同事；如果没有其他参会人，可以选择“仅自己参会”。"
}

func (m *Meeting) handleActiveDraftTurn(ctx context.Context, user domain.User, conversationID, question, source string) (bool, domain.Message, error) {
	draft, err := m.repo.LatestMeetingBookingDraft(ctx, user.ID, conversationID, m.now())
	if errors.Is(err, store.ErrNotFound) {
		return false, domain.Message{}, nil
	}
	if err != nil {
		return true, domain.Message{}, err
	}
	if draft.Intent != "cancel" && (strings.Contains(question, "取消会议") || strings.Contains(question, "取消预约")) {
		return false, domain.Message{}, nil
	}
	update := MeetingBookingDraftUpdate{Version: draft.Version}
	if draft.Intent == "cancel" {
		if bookingID := m.cancellationDraftChoice(ctx, draft, question); bookingID != "" {
			update.BookingID = bookingID
		} else {
			return true, m.draftMessage(conversationID, meetingDraftPrompt(draft), &draft), nil
		}
	} else {
		if strings.TrimSpace(draft.Slots.Title) == "" {
			title := extractMeetingDraftTitle(question)
			if title != "" {
				update.Title = &title
			}
		}
		if !draft.Slots.AttendeesConfirmed {
			if meetingAttendeesExplicitlySelfOnly(question) {
				confirmed := true
				update.AttendeesConfirmed = &confirmed
				update.AttendeeUserIDs = []string{}
			} else if names := parseAttendeeNames(question); len(names) > 0 {
				attendees, clarification, resolveErr := m.resolveAttendees(ctx, user, names)
				if resolveErr != nil {
					return true, domain.Message{}, resolveErr
				}
				if clarification != "" {
					return true, m.draftMessage(conversationID, clarification+" 也可以使用人员选择器选择准确的同事。", &draft), nil
				}
				confirmed := true
				update.AttendeesConfirmed = &confirmed
				for _, attendee := range attendees {
					if attendee.UserID != user.ID {
						update.AttendeeUserIDs = append(update.AttendeeUserIDs, attendee.UserID)
					}
				}
			}
		}
	}
	result, err := m.UpdateDraft(ctx, user, draft.ID, update)
	if err != nil {
		return true, domain.Message{}, err
	}
	if result.Action != nil {
		return true, m.message(conversationID, "信息已补充完整。请选择候选会议室并确认。", result.Action), nil
	}
	return true, m.draftMessage(conversationID, meetingDraftPrompt(result.Draft), &result.Draft), nil
}

func (m *Meeting) cancellationDraftChoice(ctx context.Context, draft domain.MeetingBookingDraft, question string) string {
	if index := ordinalChoice(question); index >= 0 && index < len(draft.Slots.CandidateBookingIDs) {
		return draft.Slots.CandidateBookingIDs[index]
	}
	needle := strings.TrimSpace(question)
	for _, token := range []string{"帮我", "请", "取消", "预约", "会议室", "会议", "那个", "这场", "一下"} {
		needle = strings.ReplaceAll(needle, token, "")
	}
	needle = strings.TrimSpace(needle)
	if len([]rune(needle)) < 2 {
		return ""
	}
	matches := []string{}
	for _, bookingID := range draft.Slots.CandidateBookingIDs {
		booking, err := m.repo.GetMeetingBooking(ctx, bookingID)
		if err != nil || booking.UserID != draft.UserID || booking.Status != "active" || !booking.EndAt.After(m.now()) {
			continue
		}
		haystack := booking.Title + " " + booking.RoomName
		if strings.Contains(haystack, needle) || strings.Contains(needle, booking.Title) {
			matches = append(matches, booking.ID)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

func extractMeetingDraftTitle(question string) string {
	if index := strings.LastIndex(question, "用户补充："); index >= 0 {
		question = question[index+len("用户补充："):]
	}
	if title := parseMeetingTitle(question); title != "" {
		return trimMeetingTitle(title)
	}
	value := strings.TrimSpace(question)
	if value == "" || LooksLikeMeetingBooking(value) || meetingAttendeesExplicitlySelfOnly(value) || strings.Contains(value, "参会人") {
		return ""
	}
	return trimMeetingTitle(value)
}

func trimMeetingTitle(value string) string {
	return strings.TrimSpace(value)
}

func ordinalChoice(value string) int {
	normalized := strings.TrimSpace(value)
	for index, tokens := range [][]string{
		{"第一个", "第1个", "第一个会议", "选第一个", "选择第一个", "1", "选1"},
		{"第二个", "第2个", "第二个会议", "选第二个", "选择第二个", "2", "选2"},
		{"第三个", "第3个", "第三个会议", "选第三个", "选择第三个", "3", "选3"},
	} {
		for _, token := range tokens {
			if normalized == token || (len([]rune(token)) > 1 && strings.Contains(normalized, token)) {
				return index
			}
		}
	}
	return -1
}

func (m *Meeting) UpdateDraft(ctx context.Context, user domain.User, draftID string, input MeetingBookingDraftUpdate) (MeetingBookingDraftResult, error) {
	draft, err := m.repo.GetMeetingBookingDraft(ctx, draftID)
	if err != nil {
		return MeetingBookingDraftResult{}, err
	}
	if draft.UserID != user.ID {
		return MeetingBookingDraftResult{}, store.ErrForbidden
	}
	if draft.Status != meetingDraftCollecting || !draft.ExpiresAt.After(m.now()) {
		return MeetingBookingDraftResult{}, store.ErrConflict
	}
	if input.Version != draft.Version {
		return MeetingBookingDraftResult{}, store.ErrConflict
	}
	if draft.Intent == "cancel" {
		if input.BookingID == "" || !containsMeetingString(draft.Slots.CandidateBookingIDs, input.BookingID) {
			return MeetingBookingDraftResult{}, errors.New("请选择有效的会议预约")
		}
		draft.Slots.SelectedBookingID = input.BookingID
	} else {
		if input.Title != nil {
			title := trimMeetingTitle(*input.Title)
			if title == "" {
				return MeetingBookingDraftResult{}, errors.New("会议主题不能为空")
			}
			if len([]rune(title)) > 100 {
				return MeetingBookingDraftResult{}, errors.New("会议主题不能超过 100 个字符")
			}
			draft.Slots.Title = title
		}
		if input.AttendeesConfirmed != nil {
			if !*input.AttendeesConfirmed {
				return MeetingBookingDraftResult{}, errors.New("请确认参会人范围")
			}
			if len(input.AttendeeUserIDs)+1 > 50 {
				return MeetingBookingDraftResult{}, errors.New("每场会议最多 50 名内部参会人")
			}
			if _, err = m.resolveAttendeeIDs(ctx, user, input.AttendeeUserIDs); err != nil {
				return MeetingBookingDraftResult{}, err
			}
			draft.Slots.AttendeeUserIDs = uniqueStrings(input.AttendeeUserIDs, user.ID)
			draft.Slots.AttendeesConfirmed = true
		}
	}
	draft.MissingFields = meetingDraftMissingFields(draft)
	draft.UpdatedAt = m.now()
	if len(draft.MissingFields) > 0 {
		updated, updateErr := m.repo.UpdateMeetingBookingDraft(ctx, draft, input.Version)
		updated = m.enrichDraft(ctx, updated)
		return MeetingBookingDraftResult{Draft: updated}, updateErr
	}
	if draft.Intent == "cancel" {
		action, actionErr := m.buildCancellationAction(ctx, user, draft.Slots.SelectedBookingID, draft.Channel, draft.ConversationID)
		if actionErr != nil {
			return MeetingBookingDraftResult{}, actionErr
		}
		draft.UpdatedAt = m.now()
		updated, updateErr := m.repo.CompleteMeetingBookingDraft(ctx, draft, action, input.Version, m.now())
		if updateErr == nil {
			m.audit(ctx, user, "meeting.action.create", "meeting_booking_action", action.ID, map[string]any{"intent": "cancel", "booking_id": action.BookingID, "draft_id": draft.ID})
		}
		return MeetingBookingDraftResult{Draft: updated, Action: &action}, updateErr
	}
	action, actionErr := m.actionFromDraft(ctx, user, draft)
	if actionErr != nil {
		return MeetingBookingDraftResult{}, actionErr
	}
	draft.UpdatedAt = m.now()
	updated, updateErr := m.repo.CompleteMeetingBookingDraft(ctx, draft, action, input.Version, m.now())
	if updateErr == nil {
		m.audit(ctx, user, "meeting.action.create", "meeting_booking_action", action.ID, map[string]any{"intent": action.Intent, "options": len(action.Options), "draft_id": draft.ID})
	}
	return MeetingBookingDraftResult{Draft: updated, Action: &action}, updateErr
}

func (m *Meeting) actionFromDraft(ctx context.Context, user domain.User, draft domain.MeetingBookingDraft) (domain.MeetingBookingAction, error) {
	date, err := time.ParseInLocation("2006-01-02", draft.Slots.Date, m.location)
	if err != nil {
		return domain.MeetingBookingAction{}, errors.New("会议日期无效，请重新发起预约")
	}
	attendees, err := m.resolveAttendeeIDs(ctx, user, draft.Slots.AttendeeUserIDs)
	if err != nil {
		return domain.MeetingBookingAction{}, err
	}
	request := meetingRequest{Intent: "create", Query: draft.Slots.OriginalQuery, Date: date, Times: []clockTime{{Hour: draft.Slots.Hour, Minute: draft.Slots.Minute}}, Duration: time.Duration(draft.Slots.DurationMinutes) * time.Minute, Title: draft.Slots.Title, Capacity: draft.Slots.Capacity}
	options, requestedRoom, err := m.candidates(ctx, request, request.Query, attendees)
	if err != nil {
		return domain.MeetingBookingAction{}, err
	}
	if len(options) == 0 {
		return domain.MeetingBookingAction{}, errors.New("指定当天没有同时满足会议室和参会人空闲的方案")
	}
	now := m.now()
	action := domain.MeetingBookingAction{ID: ids.New("mba"), UserID: user.ID, Intent: "create", Title: draft.Slots.Title, Attendees: attendees, Capacity: maxInt(draft.Slots.Capacity, len(attendees)), RequestedRoomName: requestedRoom, Options: options, Status: domain.MeetingActionPending, SourceChannel: draft.Channel, SourceConversationID: draft.ConversationID, ExpiresAt: now.Add(15 * time.Minute), CreatedAt: now}
	return action, nil
}

func (m *Meeting) resolveAttendeeIDs(ctx context.Context, requester domain.User, userIDs []string) ([]domain.MeetingAttendee, error) {
	users, err := m.repo.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]domain.User, len(users))
	for _, value := range users {
		byID[value.ID] = value
	}
	idsToResolve := uniqueStrings(userIDs, requester.ID)
	attendees := []domain.MeetingAttendee{{UserID: requester.ID, OpenID: requester.FeishuOpenID, Name: requester.Name}}
	for _, userID := range idsToResolve {
		if userID == requester.ID {
			continue
		}
		value, ok := byID[userID]
		if !ok || value.Status != "active" || value.FeishuOpenID == "" {
			return nil, errors.New("所选参会人已离职、不可见或缺少飞书身份，请重新选择")
		}
		attendees = append(attendees, domain.MeetingAttendee{UserID: value.ID, OpenID: value.FeishuOpenID, Name: value.Name})
	}
	if requester.FeishuOpenID == "" {
		return nil, errors.New("当前账号缺少飞书身份，无法预约会议室")
	}
	return attendees, nil
}

func (m *Meeting) SearchAttendees(ctx context.Context, user domain.User, query string, limit int) ([]domain.MeetingAttendeeOption, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	users, err := m.repo.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	values := []domain.MeetingAttendeeOption{}
	for _, value := range users {
		if value.Status != "active" || value.FeishuOpenID == "" || value.ID == user.ID {
			continue
		}
		haystack := strings.ToLower(value.Name + " " + value.JobTitle + " " + strings.Join(value.DepartmentIDs, " "))
		if query != "" && !strings.Contains(haystack, query) {
			continue
		}
		values = append(values, domain.MeetingAttendeeOption{ID: value.ID, Name: value.Name, AvatarURL: value.AvatarURL, DepartmentIDs: value.DepartmentIDs, JobTitle: value.JobTitle})
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Name < values[j].Name })
	if len(values) > limit {
		values = values[:limit]
	}
	return values, nil
}

func (m *Meeting) ExpireDrafts(ctx context.Context, user domain.User, conversationID string) error {
	return m.repo.ExpireMeetingBookingDrafts(ctx, user.ID, conversationID, m.now())
}

func (m *Meeting) PrepareCancellationForBooking(ctx context.Context, user domain.User, bookingID, source, conversationID string) (domain.MeetingBookingAction, error) {
	action, err := m.buildCancellationAction(ctx, user, bookingID, source, conversationID)
	if err != nil {
		return action, err
	}
	if err = m.repo.CreateMeetingBookingAction(ctx, action); err != nil {
		return action, err
	}
	m.audit(ctx, user, "meeting.action.create", "meeting_booking_action", action.ID, map[string]any{"intent": "cancel", "booking_id": bookingID})
	return action, nil
}

func (m *Meeting) buildCancellationAction(ctx context.Context, user domain.User, bookingID, source, conversationID string) (domain.MeetingBookingAction, error) {
	booking, err := m.repo.GetMeetingBooking(ctx, bookingID)
	if err != nil {
		return domain.MeetingBookingAction{}, err
	}
	if booking.UserID != user.ID {
		return domain.MeetingBookingAction{}, store.ErrForbidden
	}
	if booking.Status != "active" || !booking.EndAt.After(m.now()) {
		return domain.MeetingBookingAction{}, store.ErrConflict
	}
	now := m.now()
	action := domain.MeetingBookingAction{ID: ids.New("mba"), UserID: user.ID, Intent: "cancel", Title: booking.Title, Attendees: booking.Attendees, Capacity: len(booking.Attendees), BookingID: booking.ID, Status: domain.MeetingActionPending, SourceChannel: source, SourceConversationID: conversationID, ExpiresAt: now.Add(15 * time.Minute), CreatedAt: now}
	return action, nil
}

func (m *Meeting) cancellationDraft(ctx context.Context, user domain.User, conversationID, source string, bookings []domain.MeetingBooking) (domain.Message, error) {
	now := m.now()
	idsList := make([]string, 0, len(bookings))
	for _, booking := range bookings {
		idsList = append(idsList, booking.ID)
	}
	draft := domain.MeetingBookingDraft{ID: ids.New("mbd"), UserID: user.ID, ConversationID: conversationID, Channel: source, Intent: "cancel", Slots: domain.MeetingBookingDraftSlots{CandidateBookingIDs: idsList}, MissingFields: []string{"booking"}, Status: meetingDraftCollecting, Version: 1, BookingChoices: bookings, ExpiresAt: now.Add(15 * time.Minute), CreatedAt: now, UpdatedAt: now}
	if err := m.repo.CreateMeetingBookingDraft(ctx, draft); err != nil {
		return domain.Message{}, err
	}
	return m.draftMessage(conversationID, meetingDraftPrompt(draft), &draft), nil
}

func (m *Meeting) enrichDraft(ctx context.Context, draft domain.MeetingBookingDraft) domain.MeetingBookingDraft {
	if draft.Intent == "cancel" && len(draft.BookingChoices) == 0 {
		for _, bookingID := range draft.Slots.CandidateBookingIDs {
			if booking, err := m.repo.GetMeetingBooking(ctx, bookingID); err == nil && booking.UserID == draft.UserID {
				draft.BookingChoices = append(draft.BookingChoices, booking)
			}
		}
	}
	if draft.Intent == "create" && len(draft.AttendeeChoices) == 0 && len(draft.Slots.AttendeeUserIDs) > 0 {
		if users, err := m.repo.ListUsers(ctx); err == nil {
			selected := make(map[string]bool, len(draft.Slots.AttendeeUserIDs))
			for _, userID := range draft.Slots.AttendeeUserIDs {
				selected[userID] = true
			}
			for _, value := range users {
				if selected[value.ID] {
					draft.AttendeeChoices = append(draft.AttendeeChoices, domain.MeetingAttendeeOption{ID: value.ID, Name: value.Name, AvatarURL: value.AvatarURL, DepartmentIDs: value.DepartmentIDs, JobTitle: value.JobTitle})
				}
			}
		}
	}
	return draft
}

func (m *Meeting) GetDraft(ctx context.Context, user domain.User, id string) (domain.MeetingBookingDraft, error) {
	draft, err := m.repo.GetMeetingBookingDraft(ctx, id)
	if err == nil && draft.UserID != user.ID {
		return domain.MeetingBookingDraft{}, store.ErrForbidden
	}
	return m.enrichDraft(ctx, draft), err
}

func uniqueStrings(values []string, exclude string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || value == exclude || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func containsMeetingString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (m *Meeting) draftMessage(conversationID, content string, draft *domain.MeetingBookingDraft) domain.Message {
	return domain.Message{ID: ids.New("msg"), ConversationID: conversationID, Role: "assistant", Content: content, Citations: []domain.Citation{}, MeetingBookingDraft: draft, CreatedAt: m.now()}
}

func (m *Meeting) cancellationChoiceLabel(booking domain.MeetingBooking) string {
	return fmt.Sprintf("%s · %s · %s", booking.Title, meetingTimeLabel(booking.StartAt.In(m.location), booking.EndAt.In(m.location)), booking.RoomName)
}
