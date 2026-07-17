package service

import (
	"context"
	"encoding/json"
	"errors"
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

type FeishuBot struct {
	repo      store.Repository
	chat      *Chat
	sender    FeishuMessageSender
	appLink   string
	reminders *Reminder
}

func NewFeishuBot(repo store.Repository, chat *Chat, sender FeishuMessageSender, appLink string, reminders ...*Reminder) *FeishuBot {
	bot := &FeishuBot{repo: repo, chat: chat, sender: sender, appLink: strings.TrimSpace(appLink)}
	if len(reminders) > 0 {
		bot.reminders = reminders[0]
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
	if b.reminders != nil && (LooksLikeReminder(question) || isConfirmText(question) || isCancelText(question)) {
		now := time.Now()
		_, err = b.repo.CreateReminderBotJob(ctx, domain.ReminderBotJob{ID: ids.New("rbj"), EventID: event.EventID, OpenID: event.OpenID, ChatID: event.ChatID, MessageID: event.MessageID, Content: question, Status: "pending", AvailableAt: now, CreatedAt: now, UpdatedAt: now})
		if err != nil {
			b.repo.ForgetProcessedEvent(ctx, event.EventID)
		}
		return err
	}
	go b.answer(event, question)
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

func (b *FeishuBot) answer(event feishu.MessageEvent, question string) {
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
	conv, err := b.chat.CreateConversation(ctx, user)
	if err != nil {
		slog.Error("create feishu conversation failed", "event_id", event.EventID, "error", err)
		return
	}
	runID, err := b.chat.StartFrom(ctx, user, conv.ID, question, "feishu_bot")
	if err != nil {
		slog.Error("start feishu agent run failed", "event_id", event.EventID, "error", err)
		return
	}
	history, events, found := b.chat.Subscribe(runID)
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
	if b.appLink != "" {
		answer += "\n\n在 Agent 中查看完整引用：" + b.appLink
	}
	if _, err = b.sender.SendText(ctx, "chat_id", event.ChatID, answer, event.EventID); err != nil {
		slog.Error("send feishu answer failed", "event_id", event.EventID, "error", err)
	}
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
