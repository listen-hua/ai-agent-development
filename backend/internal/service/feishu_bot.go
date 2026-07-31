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

type FeishuBot struct {
	repo        store.Repository
	chat        *Chat
	sender      FeishuMessageSender
	appLink     string
	reminders   *Reminder
	meetings    *Meeting
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
	if b.appLink != "" {
		answer += "\n\n在 Agent 中查看完整引用：" + b.appLink
	}
	if _, err = b.sender.SendText(ctx, "chat_id", event.ChatID, answer, event.EventID); err != nil {
		slog.Error("send feishu answer failed", "event_id", event.EventID, "error", err)
	}
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
