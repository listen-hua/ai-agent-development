package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/store"
)

type ReminderDeliverySender interface {
	Configured() bool
	SendReminder(context.Context, string, string, string, string, string) (string, error)
	SendReminderConfirmation(context.Context, string, string, string, string, string) (string, error)
	SendText(context.Context, string, string, string, string) (string, error)
}

type ReminderDispatcher struct {
	repo      store.Repository
	reminders *Reminder
	sender    ReminderDeliverySender
	appLink   string
	grace     time.Duration
	retention time.Duration
	lastClean time.Time
	now       func() time.Time
}

func NewReminderDispatcher(repo store.Repository, reminders *Reminder, sender ReminderDeliverySender, appLink string, grace time.Duration, retention ...time.Duration) *ReminderDispatcher {
	if grace <= 0 {
		grace = 30 * time.Minute
	}
	retentionDuration := 90 * 24 * time.Hour
	if len(retention) > 0 && retention[0] > 0 {
		retentionDuration = retention[0]
	}
	return &ReminderDispatcher{repo: repo, reminders: reminders, sender: sender, appLink: appLink, grace: grace, retention: retentionDuration, now: time.Now}
}

func (d *ReminderDispatcher) Tick(ctx context.Context) error {
	now := d.now()
	if d.lastClean.IsZero() || now.Sub(d.lastClean) >= time.Hour {
		if err := d.repo.CleanupReminderHistory(ctx, now.Add(-d.retention)); err != nil {
			return err
		}
		if err := d.repo.CleanupConversationHistory(ctx, now.Add(-d.retention)); err != nil {
			return err
		}
		d.lastClean = now
	}
	if err := d.materializeDue(ctx); err != nil {
		return err
	}
	if err := d.deliver(ctx); err != nil {
		return err
	}
	return d.processBotJobs(ctx)
}

func (d *ReminderDispatcher) materializeDue(ctx context.Context) error {
	now := d.now()
	values, err := d.repo.ListDueReminders(ctx, now, 100)
	if err != nil {
		return err
	}
	for _, reminder := range values {
		if reminder.NextFireAt == nil {
			continue
		}
		scheduledFor := *reminder.NextFireAt
		var nextFireAt *time.Time
		if reminder.Schedule.Type != domain.ReminderOnce {
			next, nextErr := d.reminders.NextOccurrence(ctx, reminder.Schedule, scheduledFor)
			if nextErr != nil {
				_ = d.repo.UpdateReminderRuntime(ctx, reminder.ID, "failed", nil, nextErr.Error(), now)
				continue
			}
			nextFireAt = &next
		}
		delivery, created, createErr := d.repo.CreateReminderDeliveryAndAdvance(ctx, reminder.ID, scheduledFor, nextFireAt, now)
		if createErr != nil {
			return createErr
		}
		if !created {
			continue
		}
		if reminder.Schedule.Type == domain.ReminderWorkday && !d.reminders.IsWorkday(ctx, scheduledFor) {
			delivery.Status = "missed"
			delivery.Error = "当天已被公司日历标记为非工作日"
			delivery.UpdatedAt = now
			delivery.LockedUntil = nil
			_ = d.repo.UpdateReminderDelivery(ctx, delivery)
		}
	}
	return nil
}

func (d *ReminderDispatcher) deliver(ctx context.Context) error {
	now := d.now()
	values, err := d.repo.ClaimReminderDeliveries(ctx, now, 2*time.Minute, 50)
	if err != nil {
		return err
	}
	for _, delivery := range values {
		d.processDelivery(ctx, delivery)
	}
	return nil
}

func (d *ReminderDispatcher) processDelivery(ctx context.Context, delivery domain.ReminderDelivery) {
	now := d.now()
	reminder, err := d.repo.GetReminder(ctx, delivery.ReminderID)
	if err != nil {
		d.failDelivery(ctx, delivery, err, true)
		return
	}
	user, err := d.repo.GetUser(ctx, reminder.UserID)
	if err != nil || user.Status != "active" || user.FeishuOpenID == "" {
		if err == nil {
			err = fmt.Errorf("用户不可用或缺少飞书 open_id")
		}
		d.failDelivery(ctx, delivery, err, true)
		return
	}
	if now.Sub(delivery.ScheduledFor) > d.grace {
		delivery.Status, delivery.Error, delivery.LockedUntil, delivery.UpdatedAt = "missed", "超过30分钟补发窗口", nil, now
		_ = d.repo.UpdateReminderDelivery(ctx, delivery)
		status := ""
		if reminder.Schedule.Type == domain.ReminderOnce {
			status = "completed"
		}
		_ = d.repo.UpdateReminderRuntime(ctx, reminder.ID, status, nil, delivery.Error, now)
		return
	}
	if d.sender == nil || !d.sender.Configured() {
		d.failDelivery(ctx, delivery, fmt.Errorf("飞书机器人未配置"), true)
		return
	}
	scheduleText := "计划时间：" + delivery.ScheduledFor.In(d.reminders.location).Format("2006-01-02 15:04")
	if now.Sub(delivery.ScheduledFor) > time.Minute {
		scheduleText += "（延迟提醒）"
	}
	messageID, sendErr := d.sender.SendReminder(ctx, user.FeishuOpenID, reminder.Content, scheduleText, d.appLink, delivery.IdempotencyKey)
	if sendErr != nil {
		d.failDelivery(ctx, delivery, sendErr, isPermanentFeishuError(sendErr))
		return
	}
	delivery.Status, delivery.MessageID, delivery.Error, delivery.LockedUntil, delivery.UpdatedAt = "sent", messageID, "", nil, now
	_ = d.repo.UpdateReminderDelivery(ctx, delivery)
	status := ""
	if reminder.Schedule.Type == domain.ReminderOnce {
		status = "completed"
	}
	_ = d.repo.UpdateReminderRuntime(ctx, reminder.ID, status, &now, "", now)
}

func (d *ReminderDispatcher) failDelivery(ctx context.Context, delivery domain.ReminderDelivery, err error, permanent bool) {
	now := d.now()
	delivery.Error, delivery.LockedUntil, delivery.UpdatedAt = err.Error(), nil, now
	if !permanent && delivery.Attempts < 5 && now.Before(delivery.ScheduledFor.Add(d.grace)) {
		backoffs := []time.Duration{30 * time.Second, 2 * time.Minute, 5 * time.Minute, 15 * time.Minute}
		index := delivery.Attempts - 1
		if index < 0 {
			index = 0
		}
		if index >= len(backoffs) {
			index = len(backoffs) - 1
		}
		delivery.Status = "retry"
		delivery.NextAttemptAt = now.Add(backoffs[index])
		_ = d.repo.UpdateReminderDelivery(ctx, delivery)
		return
	}
	delivery.Status = "failed"
	_ = d.repo.UpdateReminderDelivery(ctx, delivery)
	reminder, getErr := d.repo.GetReminder(ctx, delivery.ReminderID)
	if getErr == nil {
		status := "paused"
		if reminder.Schedule.Type == domain.ReminderOnce {
			status = "failed"
		}
		_ = d.repo.UpdateReminderRuntime(ctx, reminder.ID, status, nil, delivery.Error, now)
	}
}

func isPermanentFeishuError(err error) bool {
	var apiErr *feishu.APIError
	if errors.As(err, &apiErr) {
		if apiErr.HTTPStatus == http.StatusTooManyRequests || apiErr.HTTPStatus >= 500 {
			return false
		}
		switch apiErr.Code {
		case 99991400, 99991401:
			return false
		default:
			return true
		}
	}
	return false
}

func (d *ReminderDispatcher) processBotJobs(ctx context.Context) error {
	now := d.now()
	jobs, err := d.repo.ClaimReminderBotJobs(ctx, now, 2*time.Minute, 20)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		d.processBotJob(ctx, job)
	}
	return nil
}

func (d *ReminderDispatcher) processBotJob(ctx context.Context, job domain.ReminderBotJob) {
	now := d.now()
	user, err := d.repo.GetUserByOpenID(ctx, job.OpenID)
	if err != nil {
		user, err = d.repo.UpsertUser(ctx, domain.User{FeishuOpenID: job.OpenID, Name: "飞书用户", Status: "active", Roles: []domain.Role{domain.RoleEmployee}})
	}
	if err == nil {
		var conversation domain.Conversation
		conversation, err = d.createBotConversation(ctx, user, job.Content)
		if err == nil {
			var handled bool
			var message domain.Message
			handled, message, err = d.reminders.HandleChat(ctx, user, conversation.ID, job.Content, "feishu_bot")
			if err == nil && handled {
				_ = d.repo.AddMessage(ctx, message)
				if message.ReminderAction != nil {
					_, err = d.sender.SendReminderConfirmation(ctx, user.FeishuOpenID, "确认个人提醒", message.Content, message.ReminderAction.ID, job.EventID)
				} else {
					_, err = d.sender.SendText(ctx, "open_id", user.FeishuOpenID, message.Content, job.EventID)
				}
			}
		}
	}
	job.UpdatedAt = now
	if err == nil {
		job.Status, job.LastError = "done", ""
	} else if job.Attempts < 3 {
		job.Status, job.LastError, job.AvailableAt = "retry", err.Error(), now.Add(time.Duration(job.Attempts)*time.Minute)
	} else {
		job.Status, job.LastError = "failed", err.Error()
	}
	_ = d.repo.UpdateReminderBotJob(ctx, job)
}

func (d *ReminderDispatcher) createBotConversation(ctx context.Context, user domain.User, content string) (domain.Conversation, error) {
	now := d.now()
	conversation := domain.Conversation{ID: ids.New("conv"), UserID: user.ID, Title: "个人提醒", CreatedAt: now, UpdatedAt: now}
	if err := d.repo.CreateConversation(ctx, conversation); err != nil {
		return conversation, err
	}
	message := domain.Message{ID: ids.New("msg"), ConversationID: conversation.ID, Role: "user", Content: content, Citations: []domain.Citation{}, CreatedAt: now}
	return conversation, d.repo.AddMessage(ctx, message)
}
