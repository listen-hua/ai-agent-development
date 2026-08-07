package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type MeetingRoomInfo struct {
	RoomID          string
	Name            string
	Capacity        int
	RoomLevelID     string
	Path            []string
	Enabled         bool
	ScheduleEnabled bool
	DisableStart    *time.Time
	DisableEnd      *time.Time
	DisableReason   string
}

type MeetingRoomReserveConfig struct {
	ApprovalSwitch        int
	ApprovalCondition     int
	ApprovalDurationHours float64
	StartSeconds          int
	EndSeconds            int
	MaxDurationHours      int
}

type BusyInterval struct {
	Start      time.Time
	End        time.Time
	RSVPStatus string
}

type CalendarEventInfo struct {
	EventID string
	AppLink string
}

type EventAttendeeInput struct {
	Type      string
	UserID    string
	RoomID    string
	OperateID string
}

type EventAttendee struct {
	Type       string
	UserID     string
	RoomID     string
	RSVPStatus string
}

type MeetingBookingChoice struct {
	ID    string
	Label string
}

type MeetingDraftBookingChoice struct {
	ID    string
	Label string
}

type MeetingBookingDraftCard struct {
	ID                string
	Intent            string
	Title             string
	RequesterOpenID   string
	SelectableOpenIDs []string
	SelectedOpenIDs   []string
	BookingChoices    []MeetingDraftBookingChoice
}

func (c *Client) ListMeetingRooms(ctx context.Context) ([]MeetingRoomInfo, error) {
	if !c.Configured() {
		return nil, errors.New("feishu is not configured")
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return nil, err
	}
	values := []MeetingRoomInfo{}
	pageToken := ""
	for page := 0; page < 100; page++ {
		endpoint := c.baseURL + "/open-apis/vc/v1/rooms?page_size=100&user_id_type=open_id"
		if pageToken != "" {
			endpoint += "&page_token=" + url.QueryEscape(pageToken)
		}
		var output struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Rooms []struct {
					RoomID      string   `json:"room_id"`
					Name        string   `json:"name"`
					Capacity    int      `json:"capacity"`
					RoomLevelID string   `json:"room_level_id"`
					Path        []string `json:"path"`
					RoomStatus  struct {
						Status           *bool  `json:"status"`
						ScheduleStatus   *bool  `json:"schedule_status"`
						DisableStartTime string `json:"disable_start_time"`
						DisableEndTime   string `json:"disable_end_time"`
						DisableReason    string `json:"disable_reason"`
					} `json:"room_status"`
				} `json:"rooms"`
				HasMore   bool   `json:"has_more"`
				PageToken string `json:"page_token"`
			} `json:"data"`
		}
		if err = c.getJSON(ctx, endpoint, token, &output); err != nil {
			return nil, err
		}
		if output.Code != 0 {
			return nil, &APIError{Code: output.Code, Message: output.Msg}
		}
		for _, room := range output.Data.Rooms {
			enabled, scheduleEnabled := true, true
			if room.RoomStatus.Status != nil {
				enabled = *room.RoomStatus.Status
			}
			if room.RoomStatus.ScheduleStatus != nil {
				scheduleEnabled = *room.RoomStatus.ScheduleStatus
			}
			values = append(values, MeetingRoomInfo{RoomID: room.RoomID, Name: room.Name, Capacity: room.Capacity, RoomLevelID: room.RoomLevelID, Path: room.Path, Enabled: enabled, ScheduleEnabled: scheduleEnabled, DisableStart: unixTime(room.RoomStatus.DisableStartTime), DisableEnd: unixTime(room.RoomStatus.DisableEndTime), DisableReason: room.RoomStatus.DisableReason})
		}
		if !output.Data.HasMore || output.Data.PageToken == "" {
			break
		}
		pageToken = output.Data.PageToken
	}
	return values, nil
}

func (c *Client) GetMeetingRoomReserveConfig(ctx context.Context, roomID string) (MeetingRoomReserveConfig, error) {
	if !c.Configured() {
		return MeetingRoomReserveConfig{}, errors.New("feishu is not configured")
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return MeetingRoomReserveConfig{}, err
	}
	query := url.Values{"scope_id": {roomID}, "scope_type": {"2"}, "user_id_type": {"open_id"}}
	var output struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ApproveConfig struct {
				ApprovalSwitch    int     `json:"approval_switch"`
				ApprovalCondition int     `json:"approval_condition"`
				MeetingDuration   float64 `json:"meeting_duration"`
			} `json:"approve_config"`
			TimeConfig struct {
				TimeSwitch  int    `json:"time_switch"`
				StartTime   string `json:"start_time"`
				EndTime     string `json:"end_time"`
				MaxDuration int    `json:"max_duration"`
			} `json:"time_config"`
		} `json:"data"`
	}
	if err = c.getJSON(ctx, c.baseURL+"/open-apis/vc/v1/reserve_configs/reserve_scope?"+query.Encode(), token, &output); err != nil {
		return MeetingRoomReserveConfig{}, err
	}
	if output.Code != 0 {
		return MeetingRoomReserveConfig{}, &APIError{Code: output.Code, Message: output.Msg}
	}
	startSeconds, _ := strconv.Atoi(output.Data.TimeConfig.StartTime)
	endSeconds, _ := strconv.Atoi(output.Data.TimeConfig.EndTime)
	if startSeconds <= 0 {
		startSeconds = 9 * 3600
	}
	if endSeconds <= startSeconds {
		endSeconds = 18 * 3600
	}
	maxDuration := output.Data.TimeConfig.MaxDuration
	if maxDuration <= 0 {
		maxDuration = 2
	}
	return MeetingRoomReserveConfig{ApprovalSwitch: output.Data.ApproveConfig.ApprovalSwitch, ApprovalCondition: output.Data.ApproveConfig.ApprovalCondition, ApprovalDurationHours: output.Data.ApproveConfig.MeetingDuration, StartSeconds: startSeconds, EndSeconds: endSeconds, MaxDurationHours: maxDuration}, nil
}

func (c *Client) MeetingRoomFreeBusy(ctx context.Context, roomIDs []string, start, end time.Time) (map[string][]BusyInterval, error) {
	values := make(map[string][]BusyInterval, len(roomIDs))
	if len(roomIDs) == 0 {
		return values, nil
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return nil, err
	}
	query := url.Values{"time_min": {start.Format(time.RFC3339)}, "time_max": {end.Format(time.RFC3339)}}
	for _, roomID := range roomIDs {
		query.Add("room_ids", roomID)
	}
	var output struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ErrorRoomIDs []string `json:"error_room_ids"`
			FreeBusy     map[string][]struct {
				StartTime string `json:"start_time"`
				EndTime   string `json:"end_time"`
			} `json:"free_busy"`
		} `json:"data"`
	}
	if err = c.getJSON(ctx, c.baseURL+"/open-apis/meeting_room/freebusy/batch_get?"+query.Encode(), token, &output); err != nil {
		return nil, err
	}
	if output.Code != 0 {
		return nil, &APIError{Code: output.Code, Message: output.Msg}
	}
	if len(output.Data.ErrorRoomIDs) > 0 {
		return nil, fmt.Errorf("feishu meeting room freebusy rejected room ids: %s", strings.Join(output.Data.ErrorRoomIDs, ","))
	}
	for roomID, intervals := range output.Data.FreeBusy {
		for _, interval := range intervals {
			from, fromErr := time.Parse(time.RFC3339, interval.StartTime)
			to, toErr := time.Parse(time.RFC3339, interval.EndTime)
			if fromErr == nil && toErr == nil {
				values[roomID] = append(values[roomID], BusyInterval{Start: from, End: to})
			}
		}
	}
	return values, nil
}

func (c *Client) UserFreeBusy(ctx context.Context, openIDs []string, start, end time.Time) (map[string][]BusyInterval, error) {
	values := make(map[string][]BusyInterval, len(openIDs))
	if len(openIDs) == 0 {
		return values, nil
	}
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return nil, err
	}
	var output struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			FreebusyLists []struct {
				UserID        string `json:"user_id"`
				FreebusyItems []struct {
					StartTime string `json:"start_time"`
					EndTime   string `json:"end_time"`
					RSVP      string `json:"rsvp_status"`
				} `json:"freebusy_items"`
			} `json:"freebusy_lists"`
		} `json:"data"`
	}
	input := map[string]any{"time_min": start.Format(time.RFC3339), "time_max": end.Format(time.RFC3339), "user_ids": openIDs, "include_external_calendar": true, "only_busy": true, "need_rsvp_status": true}
	if err = c.postJSON(ctx, c.baseURL+"/open-apis/calendar/v4/freebusy/batch?user_id_type=open_id", input, token, &output); err != nil {
		return nil, err
	}
	if output.Code != 0 {
		return nil, &APIError{Code: output.Code, Message: output.Msg}
	}
	for _, user := range output.Data.FreebusyLists {
		for _, interval := range user.FreebusyItems {
			from, fromErr := time.Parse(time.RFC3339, interval.StartTime)
			to, toErr := time.Parse(time.RFC3339, interval.EndTime)
			if fromErr == nil && toErr == nil {
				values[user.UserID] = append(values[user.UserID], BusyInterval{Start: from, End: to, RSVPStatus: interval.RSVP})
			}
		}
	}
	return values, nil
}

func (c *Client) CreateSharedCalendar(ctx context.Context, summary string) (string, error) {
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return "", err
	}
	var output struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Calendar struct {
				CalendarID string `json:"calendar_id"`
			} `json:"calendar"`
		} `json:"data"`
	}
	input := map[string]any{"summary": summary, "description": "由行政 AI 统一创建和管理会议室预约", "permissions": "private", "type": "shared"}
	if err = c.postJSON(ctx, c.baseURL+"/open-apis/calendar/v4/calendars", input, token, &output); err != nil {
		return "", err
	}
	if output.Code != 0 || output.Data.Calendar.CalendarID == "" {
		return "", &APIError{Code: output.Code, Message: output.Msg}
	}
	return output.Data.Calendar.CalendarID, nil
}

func (c *Client) CreateCalendarEvent(ctx context.Context, calendarID, title, description string, start, end time.Time, idempotencyKey string) (CalendarEventInfo, error) {
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return CalendarEventInfo{}, err
	}
	query := url.Values{"user_id_type": {"open_id"}, "idempotency_key": {normalizeMessageUUID(idempotencyKey)}}
	endpoint := c.baseURL + "/open-apis/calendar/v4/calendars/" + url.PathEscape(calendarID) + "/events?" + query.Encode()
	input := map[string]any{
		"summary": title, "description": description, "need_notification": false,
		"start_time": map[string]string{"timestamp": strconv.FormatInt(start.Unix(), 10), "timezone": "Asia/Shanghai"},
		"end_time":   map[string]string{"timestamp": strconv.FormatInt(end.Unix(), 10), "timezone": "Asia/Shanghai"},
		"visibility": "private", "attendee_ability": "can_see_others", "free_busy_status": "busy",
	}
	var output struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			Event struct {
				EventID string `json:"event_id"`
				AppLink string `json:"app_link"`
			} `json:"event"`
		} `json:"data"`
	}
	if err = c.postJSON(ctx, endpoint, input, token, &output); err != nil {
		return CalendarEventInfo{}, err
	}
	if output.Code != 0 || output.Data.Event.EventID == "" {
		return CalendarEventInfo{}, &APIError{Code: output.Code, Message: output.Msg}
	}
	return CalendarEventInfo{EventID: output.Data.Event.EventID, AppLink: output.Data.Event.AppLink}, nil
}

func (c *Client) AddCalendarEventAttendees(ctx context.Context, calendarID, eventID string, attendees []EventAttendeeInput, needNotification bool) ([]EventAttendee, error) {
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return nil, err
	}
	payload := make([]map[string]any, 0, len(attendees))
	for _, attendee := range attendees {
		value := map[string]any{"type": attendee.Type}
		if attendee.UserID != "" {
			value["user_id"] = attendee.UserID
		}
		if attendee.RoomID != "" {
			value["room_id"] = attendee.RoomID
		}
		if attendee.OperateID != "" {
			value["operate_id"] = attendee.OperateID
		}
		payload = append(payload, value)
	}
	endpoint := c.baseURL + "/open-apis/calendar/v4/calendars/" + url.PathEscape(calendarID) + "/events/" + url.PathEscape(eventID) + "/attendees?user_id_type=open_id"
	var output attendeeResponse
	if err = c.postJSON(ctx, endpoint, map[string]any{"attendees": payload, "need_notification": needNotification, "add_operator_to_attendee": false}, token, &output); err != nil {
		return nil, err
	}
	if output.Code != 0 {
		return nil, &APIError{Code: output.Code, Message: output.Msg}
	}
	return output.values(), nil
}

type attendeeResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Attendees []struct {
			Type       string `json:"type"`
			UserID     string `json:"user_id"`
			RoomID     string `json:"room_id"`
			RSVPStatus string `json:"rsvp_status"`
		} `json:"attendees"`
		Items []struct {
			Type       string `json:"type"`
			UserID     string `json:"user_id"`
			RoomID     string `json:"room_id"`
			RSVPStatus string `json:"rsvp_status"`
		} `json:"items"`
	} `json:"data"`
}

func (r attendeeResponse) values() []EventAttendee {
	items := r.Data.Attendees
	if len(items) == 0 {
		items = r.Data.Items
	}
	values := make([]EventAttendee, 0, len(items))
	for _, item := range items {
		values = append(values, EventAttendee{Type: item.Type, UserID: item.UserID, RoomID: item.RoomID, RSVPStatus: item.RSVPStatus})
	}
	return values
}

func (c *Client) ListCalendarEventAttendees(ctx context.Context, calendarID, eventID string) ([]EventAttendee, error) {
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return nil, err
	}
	endpoint := c.baseURL + "/open-apis/calendar/v4/calendars/" + url.PathEscape(calendarID) + "/events/" + url.PathEscape(eventID) + "/attendees?user_id_type=open_id&page_size=500"
	var output attendeeResponse
	if err = c.getJSON(ctx, endpoint, token, &output); err != nil {
		return nil, err
	}
	if output.Code != 0 {
		return nil, &APIError{Code: output.Code, Message: output.Msg}
	}
	return output.values(), nil
}

func (c *Client) DeleteCalendarEvent(ctx context.Context, calendarID, eventID string, notify bool) error {
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return err
	}
	endpoint := c.baseURL + "/open-apis/calendar/v4/calendars/" + url.PathEscape(calendarID) + "/events/" + url.PathEscape(eventID) + "?need_notification=" + strconv.FormatBool(notify)
	var output struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err = c.deleteJSON(ctx, endpoint, token, &output); err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && isMissingOrDeletedCalendarEvent(*apiErr) {
			return nil
		}
		return err
	}
	if output.Code != 0 {
		if output.Code == 193001 || output.Code == 193003 {
			return nil
		}
		return &APIError{Code: output.Code, Message: output.Msg}
	}
	return nil
}

func isMissingOrDeletedCalendarEvent(apiErr APIError) bool {
	if apiErr.Code == 193001 || apiErr.Code == 193003 {
		return true
	}
	var body struct {
		Code int `json:"code"`
	}
	return json.Unmarshal([]byte(apiErr.Message), &body) == nil && (body.Code == 193001 || body.Code == 193003)
}

func (c *Client) SendMeetingBookingConfirmation(ctx context.Context, openID, content, actionID, confirmLabel string, choices []MeetingBookingChoice, idempotencyKey string) (string, error) {
	if confirmLabel == "" {
		confirmLabel = "确认预约"
	}
	actions := []any{}
	for index, choice := range choices {
		buttonType := "default"
		if index == 0 {
			buttonType = "primary"
		}
		actions = append(actions, map[string]any{"tag": "button", "type": buttonType, "name": "meeting_booking_confirm", "text": map[string]string{"tag": "plain_text", "content": choice.Label}, "value": map[string]string{"meeting_booking_action_id": actionID, "option_id": choice.ID}})
	}
	if len(actions) == 0 {
		actions = append(actions, map[string]any{"tag": "button", "type": "primary", "name": "meeting_booking_confirm", "text": map[string]string{"tag": "plain_text", "content": confirmLabel}, "value": map[string]string{"meeting_booking_action_id": actionID}})
	}
	actions = append(actions, map[string]any{"tag": "button", "name": "meeting_booking_cancel", "text": map[string]string{"tag": "plain_text", "content": "暂不操作"}, "value": map[string]string{"meeting_booking_action_id": actionID}})
	card := map[string]any{
		"header": map[string]any{"template": "blue", "title": map[string]string{"tag": "plain_text", "content": "确认会议室预约"}},
		"elements": []any{
			map[string]any{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": content}},
			map[string]any{"tag": "action", "actions": actions},
		},
	}
	return c.sendCard(ctx, "open_id", openID, card, idempotencyKey)
}

func (c *Client) SendMeetingBookingDraft(ctx context.Context, openID, content string, draft MeetingBookingDraftCard, idempotencyKey string) (string, error) {
	if draft.Intent == "cancel" {
		elements := []any{map[string]any{"tag": "markdown", "content": content}}
		for _, choice := range draft.BookingChoices {
			elements = append(elements, map[string]any{"tag": "button", "text": map[string]string{"tag": "plain_text", "content": choice.Label}, "type": "default", "width": "fill", "behaviors": []any{map[string]any{"type": "callback", "value": map[string]string{"meeting_booking_draft_id": draft.ID, "booking_id": choice.ID}}}, "name": "meeting_booking_draft_select"})
		}
		card := map[string]any{"schema": "2.0", "config": map[string]any{"update_multi": true}, "header": map[string]any{"template": "orange", "title": map[string]string{"tag": "plain_text", "content": "选择要取消的会议"}}, "body": map[string]any{"elements": elements}}
		return c.sendCard(ctx, "open_id", openID, card, idempotencyKey)
	}
	selected := make(map[string]bool, len(draft.SelectedOpenIDs))
	for _, value := range draft.SelectedOpenIDs {
		selected[value] = true
	}
	people := make([]map[string]any, 0, len(draft.SelectableOpenIDs))
	for _, value := range draft.SelectableOpenIDs {
		people = append(people, map[string]any{"id": value, "selected": selected[value]})
	}
	form := map[string]any{"tag": "form", "name": "meeting_booking_draft_form", "elements": []any{
		map[string]any{"tag": "input", "name": "meeting_title", "required": true, "max_length": 100, "default_value": draft.Title, "label": map[string]string{"tag": "plain_text", "content": "会议主题"}, "placeholder": map[string]string{"tag": "plain_text", "content": "例如：产品周报评审"}},
		map[string]any{"tag": "multi_select_person", "name": "meeting_attendees", "required": false, "options": people, "placeholder": map[string]string{"tag": "plain_text", "content": "搜索并选择参会同事"}, "width": "fill"},
		map[string]any{"tag": "button", "name": "meeting_booking_draft_submit", "form_action_type": "submit", "type": "primary", "text": map[string]string{"tag": "plain_text", "content": "保存并查找会议室"}, "behaviors": []any{map[string]any{"type": "callback", "value": map[string]string{"meeting_booking_draft_id": draft.ID}}}},
		map[string]any{"tag": "button", "name": "meeting_booking_draft_self", "form_action_type": "submit", "type": "default", "text": map[string]string{"tag": "plain_text", "content": "仅自己参会"}, "behaviors": []any{map[string]any{"type": "callback", "value": map[string]string{"meeting_booking_draft_id": draft.ID}}}},
	}}
	card := map[string]any{"schema": "2.0", "config": map[string]any{"update_multi": true}, "header": map[string]any{"template": "blue", "title": map[string]string{"tag": "plain_text", "content": "补充会议信息"}}, "body": map[string]any{"elements": []any{map[string]any{"tag": "markdown", "content": content}, form}}}
	return c.sendCard(ctx, "open_id", openID, card, idempotencyKey)
}

func (c *Client) SendMeetingBookingResult(ctx context.Context, openID, title, content, template, idempotencyKey string) (string, error) {
	card := map[string]any{"header": map[string]any{"template": template, "title": map[string]string{"tag": "plain_text", "content": title}}, "elements": []any{map[string]any{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": content}}}}
	return c.sendCard(ctx, "open_id", openID, card, idempotencyKey)
}

func (c *Client) SendMeetingRoomCleanupReminder(ctx context.Context, openID, content, idempotencyKey string) (string, error) {
	return c.SendMeetingBookingResult(ctx, openID, "会议室归还提醒", content, "orange", idempotencyKey)
}

func (c *Client) deleteJSON(ctx context.Context, endpoint, token string, output any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return &APIError{HTTPStatus: resp.StatusCode, Message: strings.TrimSpace(string(limited))}
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(output)
}

func unixTime(value string) *time.Time {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds <= 0 {
		return nil
	}
	parsed := time.Unix(seconds, 0)
	return &parsed
}
