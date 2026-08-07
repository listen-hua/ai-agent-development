package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/store"
)

type FeishuMessageSender interface {
	Configured() bool
	SendText(context.Context, string, string, string, string) (string, error)
}

type FeishuReminderSender interface {
	SendReminderConfirmation(context.Context, string, string, string, string, string) (string, error)
}

type FeishuMeetingSender interface {
	SendMeetingBookingConfirmation(context.Context, string, string, string, string, []feishu.MeetingBookingChoice, string) (string, error)
	SendMeetingBookingResult(context.Context, string, string, string, string, string) (string, error)
}

type FeishuMeetingDraftSender interface {
	SendMeetingBookingDraft(context.Context, string, string, feishu.MeetingBookingDraftCard, string) (string, error)
}
type FeishuMassageSender interface {
	SendMassageAction(context.Context, string, string, string, string, string) (string, error)
}

type FeishuBot struct {
	repo        store.Repository
	chat        *Chat
	sender      FeishuMessageSender
	appLink     string
	reminders   *Reminder
	meetings    *Meeting
	massage     *Massage
	permissions *PermissionResolver
	turns       *conversationLocks
}

func NewFeishuBot(repo store.Repository, chat *Chat, sender FeishuMessageSender, appLink string, features ...any) *FeishuBot {
	bot := &FeishuBot{repo: repo, chat: chat, sender: sender, appLink: strings.TrimSpace(appLink), turns: newConversationLocks()}
	for _, feature := range features {
		switch value := feature.(type) {
		case *Reminder:
			bot.reminders = value
		case *Meeting:
			bot.meetings = value
		case *Massage:
			bot.massage = value
		case *PermissionResolver:
			bot.permissions = value
		}
	}
	return bot
}

func (b *FeishuBot) HandleMessage(ctx context.Context, event feishu.MessageEvent) error {
	if event.EventID == "" || event.OpenID == "" || event.ChatID == "" {
		return errors.New("feishu message event is missing required identifiers")
	}
	if !b.repo.MarkEventProcessed(ctx, event.EventID) {
		return nil
	}
	if event.MessageType != "text" {
		return nil
	}
	question, err := parseFeishuText(event.Content, event.MentionKeys)
	if err != nil {
		return err
	}
	if question == "" {
		return nil
	}
	ticket := b.turns.Reserve(event.OpenID + "\x00" + event.ChatID + "\x00" + domain.AgentAdministrativeAssistant)
	go b.answer(event, question, ticket)
	return nil
}

func (b *FeishuBot) HandleCardAction(ctx context.Context, event feishu.CardActionEvent) (feishu.CardActionResult, error) {
	if event.EventID == "" {
		return feishu.CardActionResult{}, errors.New("feishu card action is missing event id")
	}
	if !b.repo.MarkEventProcessed(ctx, event.EventID) {
		return feishu.CardActionResult{}, nil
	}
	if event.Name == "reminder_confirm" || event.Name == "reminder_cancel" {
		result, err := b.handleReminderCardAction(ctx, event)
		if err != nil {
			b.repo.ForgetProcessedEvent(ctx, event.EventID)
		}
		return result, err
	}
	if event.Name == "meeting_booking_confirm" || event.Name == "meeting_booking_cancel" {
		result, err := b.handleMeetingCardAction(ctx, event)
		if err != nil {
			b.repo.ForgetProcessedEvent(ctx, event.EventID)
		}
		return result, err
	}
	if strings.HasPrefix(event.Name, "meeting_booking_draft_") {
		result, err := b.handleMeetingDraftCardAction(ctx, event)
		if err != nil {
			b.repo.ForgetProcessedEvent(ctx, event.EventID)
		}
		return result, err
	}
	if strings.HasPrefix(event.Name, "massage_") {
		result, err := b.handleMassageCardAction(ctx, event)
		if err != nil {
			b.repo.ForgetProcessedEvent(ctx, event.EventID)
		}
		return result, err
	}
	actorID, actorName := "", "飞书用户"
	if user, err := b.repo.GetUserByOpenID(ctx, event.OpenID); err == nil {
		actorID, actorName = user.ID, user.Name
	}
	_ = b.repo.AppendAudit(ctx, domain.AuditEvent{
		ID: ids.New("aud"), ActorID: actorID, ActorName: actorName, Action: "feishu.card_action",
		ResourceType: "feishu_message", ResourceID: event.MessageID,
		Metadata: map[string]any{"action": event.Name, "chat_id": event.ChatID}, CreatedAt: time.Now(),
	})
	return feishu.CardActionResult{ToastType: "info", ToastContent: "操作已接收"}, nil
}

func (b *FeishuBot) handleMassageCardAction(ctx context.Context, event feishu.CardActionEvent) (feishu.CardActionResult, error) {
	if b.massage == nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "按摩排号服务未启用"}, nil
	}
	user, err := b.repo.GetUserByOpenID(ctx, event.OpenID)
	if err != nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "无法识别当前用户"}, nil
	}
	if user, err = b.authorizedUser(ctx, user); err != nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "当前账号没有使用微光的权限"}, nil
	}
	if event.Name == "massage_call_accept" || event.Name == "massage_call_reject" {
		id, _ := event.Value["massage_call_id"].(string)
		action := "accept"
		text := "已接受，请前往按摩室"
		template := "green"
		if event.Name == "massage_call_reject" {
			action = "reject"
			text = "已跳过，本场不会再次叫号"
			template = "grey"
		}
		if id == "" {
			return feishu.CardActionResult{ToastType: "error", ToastContent: "叫号信息无效"}, nil
		}
		if _, err = b.massage.RespondCall(ctx, user, id, action); err != nil {
			return feishu.CardActionResult{ToastType: "error", ToastContent: "该叫号已过期或已处理", Card: massageResultCard("该叫号已失效", "grey")}, nil
		}
		return feishu.CardActionResult{ToastType: "success", ToastContent: text, Card: massageResultCard(text, template)}, nil
	}
	cycleID, _ := event.Value["massage_cycle_id"].(string)
	action := map[string]string{"massage_enroll": "enroll", "massage_decline": "decline", "massage_withdraw": "withdraw"}[event.Name]
	if cycleID == "" || action == "" {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "报名操作无效"}, nil
	}
	enrollment, err := b.massage.Respond(ctx, user, cycleID, action)
	if err != nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "报名已截止或操作已失效", Card: massageResultCard("报名操作未生效，请打开微光查看最新状态", "grey")}, nil
	}
	text := "本月已标记为不参加"
	if action == "enroll" {
		text = fmt.Sprintf("报名成功，你的排号是 %d", enrollment.QueueNumber)
	} else if action == "withdraw" {
		text = "已退出本月按摩排号"
	}
	return feishu.CardActionResult{ToastType: "success", ToastContent: text, Card: massageResultCard(text, "green")}, nil
}
func massageResultCard(content, template string) map[string]any {
	return map[string]any{"header": map[string]any{"template": template, "title": map[string]string{"tag": "plain_text", "content": "按摩排号"}}, "elements": []any{map[string]any{"tag": "div", "text": map[string]string{"tag": "plain_text", "content": content}}}}
}

func (b *FeishuBot) handleMeetingCardAction(ctx context.Context, event feishu.CardActionEvent) (feishu.CardActionResult, error) {
	if b.meetings == nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "会议室预约服务未启用"}, nil
	}
	actionID, _ := event.Value["meeting_booking_action_id"].(string)
	optionID, _ := event.Value["option_id"].(string)
	if actionID == "" {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "预约操作无效"}, nil
	}
	user, err := b.repo.GetUserByOpenID(ctx, event.OpenID)
	if err != nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "无法识别当前用户"}, nil
	}
	if user, err = b.authorizedUser(ctx, user); err != nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "当前账号没有使用微光的权限"}, nil
	}
	if event.Name == "meeting_booking_cancel" {
		if _, err = b.meetings.CancelAction(ctx, user, actionID); err != nil && !errors.Is(err, store.ErrConflict) {
			return feishu.CardActionResult{}, err
		}
		return feishu.CardActionResult{ToastType: "success", ToastContent: "已取消", Card: meetingResultCard("已取消本次会议室操作", "grey")}, nil
	}
	go b.confirmMeetingFromCard(user, actionID, optionID, event.EventID)
	return feishu.CardActionResult{ToastType: "info", ToastContent: "正在重新校验并预约", Card: meetingResultCard("正在重新校验会议室和参会人忙闲，结果稍后私聊通知你。", "blue")}, nil
}

func (b *FeishuBot) handleMeetingDraftCardAction(ctx context.Context, event feishu.CardActionEvent) (feishu.CardActionResult, error) {
	if b.meetings == nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "会议室预约服务未启用"}, nil
	}
	draftID, _ := event.Value["meeting_booking_draft_id"].(string)
	if draftID == "" {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "会议预约草稿无效"}, nil
	}
	user, err := b.repo.GetUserByOpenID(ctx, event.OpenID)
	if err != nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "无法识别当前用户"}, nil
	}
	if user, err = b.authorizedUser(ctx, user); err != nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "当前账号没有使用微光的权限"}, nil
	}
	input := MeetingBookingDraftUpdate{Version: 0}
	draft, err := b.meetings.GetDraft(ctx, user, draftID)
	if err != nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "会议预约草稿已失效"}, nil
	}
	input.Version = draft.Version
	if event.Name == "meeting_booking_draft_select" {
		input.BookingID, _ = event.Value["booking_id"].(string)
	} else {
		title := cardFormString(event.FormValue["meeting_title"])
		input.Title = &title
		confirmed := true
		input.AttendeesConfirmed = &confirmed
		if event.Name != "meeting_booking_draft_self" {
			openIDs := cardFormStrings(event.FormValue["meeting_attendees"])
			users, listErr := b.repo.ListUsers(ctx)
			if listErr != nil {
				return feishu.CardActionResult{}, listErr
			}
			byOpenID := map[string]string{}
			for _, value := range users {
				byOpenID[value.FeishuOpenID] = value.ID
			}
			unresolved := []string{}
			seen := map[string]bool{}
			for _, openID := range openIDs {
				if openID == "" || openID == user.FeishuOpenID || seen[openID] {
					continue
				}
				seen[openID] = true
				if userID := byOpenID[openID]; userID != "" && userID != user.ID {
					input.AttendeeUserIDs = append(input.AttendeeUserIDs, userID)
				} else {
					unresolved = append(unresolved, openID)
				}
			}
			if len(unresolved) > 0 {
				return feishu.CardActionResult{ToastType: "error", ToastContent: "部分参会人已离职或不在应用可见范围，请重新选择"}, nil
			}
		}
	}
	go b.completeMeetingDraftFromCard(user, draft, input, event.EventID)
	return feishu.CardActionResult{ToastType: "info", ToastContent: "正在校验会议信息", Card: meetingResultCard("正在查询参会人和会议室忙闲，结果稍后私聊通知你。", "blue")}, nil
}

func (b *FeishuBot) completeMeetingDraftFromCard(user domain.User, draft domain.MeetingBookingDraft, input MeetingBookingDraftUpdate, eventID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := b.meetings.UpdateDraft(ctx, user, draft.ID, input)
	sender, ok := b.sender.(FeishuMeetingSender)
	if !ok || user.FeishuOpenID == "" {
		return
	}
	if err != nil {
		_, _ = sender.SendMeetingBookingResult(ctx, user.FeishuOpenID, "会议信息校验失败", err.Error(), "red", "meeting-draft-failed-"+eventID)
		return
	}
	if result.Action == nil {
		if draftSender, ok := b.sender.(FeishuMeetingDraftSender); ok {
			_, _ = draftSender.SendMeetingBookingDraft(ctx, user.FeishuOpenID, meetingDraftPrompt(result.Draft), b.meetingDraftCard(ctx, user, result.Draft), "meeting-draft-again-"+eventID)
		}
		return
	}
	action := *result.Action
	choices := make([]feishu.MeetingBookingChoice, 0, len(action.Options))
	for index, option := range action.Options {
		choices = append(choices, feishu.MeetingBookingChoice{ID: option.ID, Label: fmt.Sprintf("候选%d %s %s", index+1, option.StartAt.Format("15:04"), option.RoomName)})
	}
	label := "确认预约"
	if action.Intent == "cancel" {
		label = "确认取消预约"
	}
	_, _ = sender.SendMeetingBookingConfirmation(ctx, user.FeishuOpenID, meetingActionSummary(action), action.ID, label, choices, "meeting-draft-ready-"+eventID)
}

func cardFormString(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func cardFormStrings(value any) []string {
	switch values := value.(type) {
	case []string:
		return values
	case []any:
		out := make([]string, 0, len(values))
		for _, item := range values {
			if text, ok := item.(string); ok && text != "" {
				out = append(out, text)
				continue
			}
			if person, ok := item.(map[string]any); ok {
				for _, key := range []string{"id", "open_id", "value"} {
					if text, ok := person[key].(string); ok && text != "" {
						out = append(out, text)
						break
					}
				}
			}
		}
		return out
	case map[string]any:
		for _, key := range []string{"id", "open_id", "value"} {
			if text, ok := values[key].(string); ok && text != "" {
				return []string{text}
			}
		}
	case string:
		if values != "" {
			return []string{values}
		}
	}
	return nil
}

func (b *FeishuBot) confirmMeetingFromCard(user domain.User, actionID, optionID, eventID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	_, _, err := b.meetings.Confirm(ctx, user, actionID, optionID)
	sender, ok := b.sender.(FeishuMeetingSender)
	if !ok || user.FeishuOpenID == "" {
		return
	}
	if err != nil {
		_, _ = sender.SendMeetingBookingResult(ctx, user.FeishuOpenID, "会议室预约失败", "重新校验时会议室或参会人可能已被占用，请回到行政 AI 重新选择。\n\n"+err.Error(), "red", "meeting-card-failed-"+eventID)
		return
	}
}

func meetingResultCard(content, template string) map[string]any {
	return map[string]any{"header": map[string]any{"template": template, "title": map[string]string{"tag": "plain_text", "content": "会议室预约"}}, "elements": []any{map[string]any{"tag": "div", "text": map[string]string{"tag": "plain_text", "content": content}}}}
}

func (b *FeishuBot) handleReminderCardAction(ctx context.Context, event feishu.CardActionEvent) (feishu.CardActionResult, error) {
	if b.reminders == nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "提醒服务未启用"}, nil
	}
	actionID, _ := event.Value["reminder_action_id"].(string)
	if actionID == "" {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "提醒操作无效"}, nil
	}
	user, err := b.repo.GetUserByOpenID(ctx, event.OpenID)
	if err != nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "无法识别当前用户"}, nil
	}
	if user, err = b.authorizedUser(ctx, user); err != nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "当前账号没有使用微光的权限"}, nil
	}
	if event.Name == "reminder_cancel" {
		if _, err = b.reminders.CancelAction(ctx, user, actionID); err != nil && !errors.Is(err, store.ErrConflict) {
			return feishu.CardActionResult{}, err
		}
		return feishu.CardActionResult{ToastType: "success", ToastContent: "已取消", Card: reminderResultCard("提醒操作已取消", "grey")}, nil
	}
	_, reminder, err := b.reminders.Confirm(ctx, user, actionID)
	if err != nil {
		return feishu.CardActionResult{ToastType: "error", ToastContent: "确认失败，请重新设置提醒", Card: reminderResultCard("确认已失效，请重新设置提醒", "red")}, nil
	}
	return feishu.CardActionResult{ToastType: "success", ToastContent: "提醒已创建", Card: reminderResultCard(confirmedReminderText(reminder), "green")}, nil
}

func reminderResultCard(content, template string) map[string]any {
	return map[string]any{"header": map[string]any{"template": template, "title": map[string]string{"tag": "plain_text", "content": "个人提醒"}}, "elements": []any{map[string]any{"tag": "div", "text": map[string]string{"tag": "plain_text", "content": content}}}}
}

func (b *FeishuBot) answer(event feishu.MessageEvent, question string, ticket *conversationTurnTicket) {
	ticket.Wait()
	defer ticket.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	user, err := b.repo.GetUserByOpenID(ctx, event.OpenID)
	if err != nil {
		user, err = b.repo.UpsertUser(ctx, domain.User{FeishuOpenID: event.OpenID, Name: "飞书用户", Roles: []domain.Role{domain.RoleEmployee}})
	}
	if err != nil {
		slog.Error("upsert feishu user failed", "event_id", event.EventID, "error", err)
		return
	}
	user, err = b.authorizedUser(ctx, user)
	if err != nil {
		slog.Info("feishu bot permission denied", "event_id", event.EventID, "open_id", event.OpenID, "error", err)
		if b.sender.Configured() {
			_, _ = b.sender.SendText(ctx, "chat_id", event.ChatID, "当前账号没有使用微光的权限，请联系管理员。", "permission-denied-"+event.EventID)
		}
		return
	}
	if isContextResetText(question) {
		if err = b.chat.ResetBotConversation(ctx, user, event.ChatID); err != nil {
			slog.Error("reset feishu conversation failed", "event_id", event.EventID, "error", err)
			return
		}
		if b.sender.Configured() {
			_, _ = b.sender.SendText(ctx, "chat_id", event.ChatID, "已开始新会话，之前的上下文不会继续使用。", event.EventID)
		}
		return
	}
	conv, err := b.chat.BoundBotConversation(ctx, user, event.ChatID)
	if err != nil {
		slog.Error("bind feishu conversation failed", "event_id", event.EventID, "error", err)
		return
	}
	runID, err := b.chat.StartFrom(ctx, user, conv.ID, question, "feishu_bot")
	if err != nil {
		slog.Error("start feishu agent run failed", "event_id", event.EventID, "error", err)
		return
	}
	history, events, found := b.chat.Subscribe(runID, user.ID)
	if !found {
		slog.Error("feishu agent run not found", "event_id", event.EventID, "run_id", runID)
		return
	}
	answer := ""
	var finalMessage *domain.Message
	consume := func(event domain.RunEvent) {
		switch event.Type {
		case "delta":
			answer += event.Delta
		case "error":
			answer = "生成失败，请稍后重试。"
		case "done":
			finalMessage = event.Message
		}
	}
	for _, runEvent := range history {
		consume(runEvent)
	}
	if events != nil {
		for runEvent := range events {
			consume(runEvent)
		}
	}
	if answer == "" || !b.sender.Configured() {
		return
	}
	if finalMessage != nil && finalMessage.ReminderAction != nil {
		if reminderSender, ok := b.sender.(FeishuReminderSender); ok {
			if _, err = reminderSender.SendReminderConfirmation(ctx, event.OpenID, "确认个人提醒", finalMessage.Content, finalMessage.ReminderAction.ID, event.EventID); err != nil {
				slog.Error("send reminder confirmation failed", "event_id", event.EventID, "error", err)
			}
		}
		return
	}
	if finalMessage != nil && finalMessage.MeetingBookingAction != nil {
		if meetingSender, ok := b.sender.(FeishuMeetingSender); ok {
			action := *finalMessage.MeetingBookingAction
			content := finalMessage.Content + "\n\n" + meetingActionSummary(action)
			choices := make([]feishu.MeetingBookingChoice, 0, len(action.Options))
			for index, option := range action.Options {
				choices = append(choices, feishu.MeetingBookingChoice{ID: option.ID, Label: fmt.Sprintf("候选%d %s %s", index+1, option.StartAt.Format("15:04"), option.RoomName)})
			}
			confirmLabel := "确认预约"
			if action.Intent == "cancel" {
				confirmLabel = "确认取消预约"
			}
			if _, err = meetingSender.SendMeetingBookingConfirmation(ctx, event.OpenID, content, action.ID, confirmLabel, choices, event.EventID); err != nil {
				slog.Error("send meeting booking confirmation failed", "event_id", event.EventID, "error", err)
			}
		}
		return
	}
	if finalMessage != nil && finalMessage.MeetingBookingDraft != nil {
		if draftSender, ok := b.sender.(FeishuMeetingDraftSender); ok {
			draft := *finalMessage.MeetingBookingDraft
			if _, err = draftSender.SendMeetingBookingDraft(ctx, event.OpenID, finalMessage.Content, b.meetingDraftCard(ctx, user, draft), event.EventID); err != nil {
				slog.Error("send meeting booking draft failed", "event_id", event.EventID, "error", err)
			}
		}
		return
	}
	if finalMessage != nil && finalMessage.MassageAction != nil {
		if massageSender, ok := b.sender.(FeishuMassageSender); ok {
			action := finalMessage.MassageAction
			if _, err = massageSender.SendMassageAction(ctx, event.OpenID, finalMessage.Content, action.Type, action.CycleID, event.EventID); err != nil {
				slog.Error("send massage action failed", "event_id", event.EventID, "error", err)
			}
		}
		return
	}
	if b.appLink != "" {
		answer += "\n\n在 Agent 中查看完整引用：" + b.appLink
	}
	if _, err = b.sender.SendText(ctx, "chat_id", event.ChatID, answer, event.EventID); err != nil {
		slog.Error("send feishu answer failed", "event_id", event.EventID, "error", err)
	}
}

func (b *FeishuBot) meetingDraftCard(ctx context.Context, user domain.User, draft domain.MeetingBookingDraft) feishu.MeetingBookingDraftCard {
	card := feishu.MeetingBookingDraftCard{ID: draft.ID, Intent: draft.Intent, Title: draft.Slots.Title, RequesterOpenID: user.FeishuOpenID}
	if draft.Intent == "cancel" {
		for _, booking := range draft.BookingChoices {
			card.BookingChoices = append(card.BookingChoices, feishu.MeetingDraftBookingChoice{ID: booking.ID, Label: b.meetings.cancellationChoiceLabel(booking)})
		}
		return card
	}
	users, _ := b.repo.ListUsers(ctx)
	selectedUserIDs := map[string]bool{}
	for _, userID := range draft.Slots.AttendeeUserIDs {
		selectedUserIDs[userID] = true
	}
	for _, value := range users {
		if selectedUserIDs[value.ID] && value.Status == "active" && value.FeishuOpenID != "" && value.ID != user.ID {
			card.SelectableOpenIDs = append(card.SelectableOpenIDs, value.FeishuOpenID)
			card.SelectedOpenIDs = append(card.SelectedOpenIDs, value.FeishuOpenID)
		}
	}
	for _, value := range users {
		if len(card.SelectableOpenIDs) >= 200 {
			break
		}
		if !selectedUserIDs[value.ID] && value.Status == "active" && value.FeishuOpenID != "" && value.ID != user.ID {
			card.SelectableOpenIDs = append(card.SelectableOpenIDs, value.FeishuOpenID)
		}
	}
	return card
}

func meetingActionSummary(action domain.MeetingBookingAction) string {
	if action.Intent == "cancel" {
		return "操作：取消预约"
	}
	lines := []string{}
	for index, option := range action.Options {
		lines = append(lines, fmt.Sprintf("候选 %d：%s · %s · 容量 %d 人", index+1, meetingTimeLabel(option.StartAt, option.EndAt), option.RoomName, option.Capacity))
	}
	lines = append(lines, "点击对应候选按钮确认；也可以在 Agent 页面查看并选择。")
	return strings.Join(lines, "\n")
}

func (b *FeishuBot) authorizedUser(ctx context.Context, user domain.User) (domain.User, error) {
	if b.permissions == nil {
		return user, nil
	}
	resolved, err := b.permissions.Resolve(ctx, user, "")
	if err != nil {
		return domain.User{}, err
	}
	if !resolved.HasPermission(domain.PermissionAgentUse) {
		return domain.User{}, ErrPermissionDenied
	}
	return resolved, nil
}

func parseFeishuText(content string, mentionKeys []string) (string, error) {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return "", errors.New("invalid feishu text message content")
	}
	text := payload.Text
	for _, key := range mentionKeys {
		text = strings.ReplaceAll(text, key, "")
	}
	return strings.TrimSpace(text), nil
}
