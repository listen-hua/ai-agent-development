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
	"internal-ai-agent/backend/internal/model"
	"internal-ai-agent/backend/internal/store"
)

type Reminder struct {
	repo      store.Repository
	provider  model.Provider
	modelName string
	location  *time.Location
	maxActive int
	now       func() time.Time
}

func NewReminder(repo store.Repository, provider model.Provider, modelName, timezone string, maxActive int) *Reminder {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		location, _ = time.LoadLocation("Asia/Shanghai")
	}
	if maxActive <= 0 {
		maxActive = 100
	}
	return &Reminder{repo: repo, provider: provider, modelName: modelName, location: location, maxActive: maxActive, now: time.Now}
}

func LooksLikeReminder(value string) bool {
	value = strings.TrimSpace(value)
	for _, keyword := range []string{"提醒我", "提醒一下", "提醒列表", "我的提醒", "查看提醒", "取消提醒", "删除提醒", "暂停提醒", "恢复提醒", "确认提醒", "确认创建", "别忘了", "叫我"} {
		if strings.Contains(value, keyword) {
			return true
		}
	}
	return false
}

func (r *Reminder) HandleChat(ctx context.Context, user domain.User, conversationID, input, source string) (bool, domain.Message, error) {
	trimmed := strings.TrimSpace(input)
	if isConfirmText(trimmed) {
		action, err := r.repo.LatestPendingReminderAction(ctx, user.ID, conversationID)
		if errors.Is(err, store.ErrNotFound) {
			return false, domain.Message{}, nil
		}
		if err != nil {
			return true, domain.Message{}, err
		}
		_, reminder, err := r.Confirm(ctx, user, action.ID)
		if err != nil {
			return true, r.message(conversationID, "这个提醒确认已失效或执行时间已经太近，请重新设置。", nil), nil
		}
		return true, r.message(conversationID, confirmedReminderText(reminder), nil), nil
	}
	if isCancelText(trimmed) {
		action, err := r.repo.LatestPendingReminderAction(ctx, user.ID, conversationID)
		if err == nil {
			cancelled, cancelErr := r.CancelAction(ctx, user, action.ID)
			return true, r.message(conversationID, "已取消本次提醒操作。", &cancelled), cancelErr
		}
	}
	if isListText(trimmed) {
		values, err := r.List(ctx, user)
		if err != nil {
			return true, domain.Message{}, err
		}
		return true, r.message(conversationID, reminderListText(values, r.location), nil), nil
	}
	if !LooksLikeReminder(trimmed) {
		return false, domain.Message{}, nil
	}
	parsed, err := r.interpret(ctx, trimmed, r.now().In(r.location))
	if err != nil {
		return true, r.message(conversationID, "暂时无法准确理解提醒时间，请换一种方式说明，例如“今天下午3点提醒我写周报”。", nil), nil
	}
	if parsed.Intent == "none" {
		return false, domain.Message{}, nil
	}
	if parsed.Clarification != "" {
		return true, r.message(conversationID, parsed.Clarification, nil), nil
	}
	if parsed.Intent == "create" {
		action, createErr := r.CreateAction(ctx, user, "create", "", parsed.Content, &parsed.Schedule, source, conversationID)
		if createErr != nil {
			return true, domain.Message{}, createErr
		}
		return true, r.message(conversationID, previewReminderText(action, r.location), &action), nil
	}
	if parsed.Intent == "pause" || parsed.Intent == "resume" || parsed.Intent == "delete" {
		reminder, findErr := r.findReminder(ctx, user, parsed.Target)
		if findErr != nil {
			return true, r.message(conversationID, "没有找到唯一匹配的提醒，请打开“我的提醒”进行选择。", nil), nil
		}
		action, createErr := r.CreateAction(ctx, user, parsed.Intent, reminder.ID, "", nil, source, conversationID)
		if createErr != nil {
			return true, domain.Message{}, createErr
		}
		return true, r.message(conversationID, previewReminderText(action, r.location), &action), nil
	}
	return true, r.message(conversationID, "请打开“我的提醒”修改具体时间或内容，我会在保存前再次让你确认。", nil), nil
}

func (r *Reminder) message(conversationID, content string, action *domain.ReminderActionDraft) domain.Message {
	return domain.Message{ID: ids.New("msg"), ConversationID: conversationID, Role: "assistant", Content: content, Citations: []domain.Citation{}, ReminderAction: action, CreatedAt: r.now()}
}

func (r *Reminder) CreateAction(ctx context.Context, user domain.User, action, reminderID, content string, schedule *domain.ReminderSchedule, source, conversationID string) (domain.ReminderActionDraft, error) {
	if source == "" {
		source = "h5"
	}
	now := r.now()
	recent, err := r.repo.CountReminderActionsSince(ctx, user.ID, now.Add(-time.Minute))
	if err != nil {
		return domain.ReminderActionDraft{}, err
	}
	if recent >= 10 {
		return domain.ReminderActionDraft{}, fmt.Errorf("提醒操作过于频繁，请稍后再试")
	}
	if action == "create" {
		values, err := r.repo.ListReminders(ctx, user.ID)
		if err != nil {
			return domain.ReminderActionDraft{}, err
		}
		active := 0
		for _, value := range values {
			if value.Status == "active" || value.Status == "paused" {
				active++
			}
		}
		if active >= r.maxActive {
			return domain.ReminderActionDraft{}, fmt.Errorf("最多只能保留 %d 个有效提醒", r.maxActive)
		}
	}
	if reminderID != "" {
		value, err := r.repo.GetReminder(ctx, reminderID)
		if err != nil {
			return domain.ReminderActionDraft{}, err
		}
		if value.UserID != user.ID {
			return domain.ReminderActionDraft{}, store.ErrForbidden
		}
		if action == "update" {
			if content == "" {
				content = value.Content
			}
			if schedule == nil {
				schedule = &value.Schedule
			}
		}
	}
	if action == "create" || action == "update" {
		var err error
		content, err = domain.NormalizeReminderContent(content)
		if err != nil {
			return domain.ReminderActionDraft{}, err
		}
		if schedule == nil || schedule.Validate() != nil {
			return domain.ReminderActionDraft{}, fmt.Errorf("invalid reminder schedule")
		}
	}
	value := domain.ReminderActionDraft{ID: ids.New("act"), UserID: user.ID, Action: action, ReminderID: reminderID, Content: content, Schedule: schedule, Status: "pending", SourceChannel: source, SourceConversationID: conversationID, ExpiresAt: now.Add(30 * time.Minute), CreatedAt: now}
	if err := r.repo.CreateReminderAction(ctx, value); err != nil {
		return value, err
	}
	r.audit(ctx, user, "reminder.action_draft", value.ID)
	return value, nil
}

func (r *Reminder) Confirm(ctx context.Context, user domain.User, actionID string) (domain.ReminderActionDraft, domain.Reminder, error) {
	action, err := r.repo.GetReminderAction(ctx, actionID)
	if err != nil {
		return action, domain.Reminder{}, err
	}
	if action.UserID != user.ID {
		return action, domain.Reminder{}, store.ErrForbidden
	}
	if action.Status == "confirmed" && action.ResultReminderID != "" {
		value, getErr := r.repo.GetReminder(ctx, action.ResultReminderID)
		return action, value, getErr
	}
	now := r.now()
	var schedule domain.ReminderSchedule
	if action.Action == "create" || action.Action == "update" {
		if action.Schedule == nil {
			return action, domain.Reminder{}, fmt.Errorf("missing reminder schedule")
		}
		schedule = *action.Schedule
	} else if action.ReminderID != "" {
		value, getErr := r.repo.GetReminder(ctx, action.ReminderID)
		if getErr != nil {
			return action, domain.Reminder{}, getErr
		}
		schedule = value.Schedule
	}
	var nextFireAt *time.Time
	if action.Action == "create" || action.Action == "update" || action.Action == "resume" {
		next, nextErr := r.NextOccurrence(ctx, schedule, now.Add(59*time.Second))
		if nextErr != nil {
			return action, domain.Reminder{}, nextErr
		}
		nextFireAt = &next
	}
	action, reminder, err := r.repo.ConfirmReminderAction(ctx, action.ID, user.ID, nextFireAt, now)
	if err == nil {
		r.audit(ctx, user, "reminder."+action.Action, reminder.ID)
	}
	return action, reminder, err
}

func (r *Reminder) CancelAction(ctx context.Context, user domain.User, actionID string) (domain.ReminderActionDraft, error) {
	value, err := r.repo.CancelReminderAction(ctx, actionID, user.ID, r.now())
	if err == nil {
		r.audit(ctx, user, "reminder.action_cancel", value.ID)
	}
	return value, err
}

func (r *Reminder) List(ctx context.Context, user domain.User) ([]domain.Reminder, error) {
	return r.repo.ListReminders(ctx, user.ID)
}

func (r *Reminder) Get(ctx context.Context, user domain.User, id string) (domain.Reminder, error) {
	value, err := r.repo.GetReminder(ctx, id)
	if err == nil && value.UserID != user.ID {
		return domain.Reminder{}, store.ErrForbidden
	}
	return value, err
}

func (r *Reminder) Deliveries(ctx context.Context, user domain.User, id string) ([]domain.ReminderDelivery, error) {
	if _, err := r.Get(ctx, user, id); err != nil {
		return nil, err
	}
	return r.repo.ListReminderDeliveries(ctx, id, 20)
}

func (r *Reminder) IsWorkday(ctx context.Context, date time.Time) bool {
	key := date.In(r.location).Format("2006-01-02")
	if override, err := r.repo.GetWorkdayOverride(ctx, key); err == nil {
		return override.IsWorkday
	}
	weekday := date.In(r.location).Weekday()
	return weekday >= time.Monday && weekday <= time.Friday
}

// RecalculateWorkdayReminders makes calendar edits effective immediately for
// every active workday reminder, including newly-added make-up workdays.
func (r *Reminder) RecalculateWorkdayReminders(ctx context.Context) error {
	values, err := r.repo.ListActiveWorkdayReminders(ctx)
	if err != nil {
		return err
	}
	now := r.now()
	for _, value := range values {
		next, nextErr := r.NextOccurrence(ctx, value.Schedule, now.Add(59*time.Second))
		if nextErr != nil {
			return nextErr
		}
		if updateErr := r.repo.UpdateReminderNextFire(ctx, value.ID, &next, now); updateErr != nil {
			return updateErr
		}
	}
	return nil
}

func (r *Reminder) NextOccurrence(ctx context.Context, schedule domain.ReminderSchedule, after time.Time) (time.Time, error) {
	if err := schedule.Validate(); err != nil {
		return time.Time{}, err
	}
	after = after.In(r.location)
	if schedule.Type == domain.ReminderOnce {
		if schedule.OnceAt == nil || !schedule.OnceAt.After(after) {
			return time.Time{}, fmt.Errorf("提醒时间已经过去或距离执行不足 60 秒")
		}
		return schedule.OnceAt.In(r.location), nil
	}
	hour, minute, err := parseClock(schedule.LocalTime)
	if err != nil {
		return time.Time{}, err
	}
	for offset := 0; offset < 370; offset++ {
		date := after.AddDate(0, 0, offset)
		candidate := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, r.location)
		if !candidate.After(after) {
			continue
		}
		switch schedule.Type {
		case domain.ReminderDaily:
			return candidate, nil
		case domain.ReminderWorkday:
			if r.IsWorkday(ctx, candidate) {
				return candidate, nil
			}
		case domain.ReminderWeekly:
			weekday := int(candidate.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			for _, expected := range schedule.Weekdays {
				if weekday == expected {
					return candidate, nil
				}
			}
		}
	}
	return time.Time{}, fmt.Errorf("未来一年内没有可执行日期")
}

func parseClock(value string) (int, int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0, err
	}
	return parsed.Hour(), parsed.Minute(), nil
}

func (r *Reminder) findReminder(ctx context.Context, user domain.User, target string) (domain.Reminder, error) {
	values, err := r.List(ctx, user)
	if err != nil {
		return domain.Reminder{}, err
	}
	target = strings.TrimSpace(target)
	matches := []domain.Reminder{}
	for _, value := range values {
		if value.Status == "cancelled" || value.Status == "completed" {
			continue
		}
		if target == "" || strings.Contains(value.Content, target) {
			matches = append(matches, value)
		}
	}
	if len(matches) != 1 {
		return domain.Reminder{}, store.ErrConflict
	}
	return matches[0], nil
}

func (r *Reminder) audit(ctx context.Context, user domain.User, action, resourceID string) {
	_ = r.repo.AppendAudit(ctx, domain.AuditEvent{ID: ids.New("aud"), ActorID: user.ID, ActorName: user.Name, Action: action, ResourceType: "reminder", ResourceID: resourceID, Metadata: map[string]any{}, CreatedAt: r.now()})
}

func isConfirmText(value string) bool {
	return value == "确认" || value == "确认提醒" || value == "确认创建" || value == "确认执行"
}
func isCancelText(value string) bool {
	return value == "取消" || value == "取消操作" || value == "取消创建"
}
func isListText(value string) bool {
	return strings.Contains(value, "提醒列表") || strings.Contains(value, "我的提醒") || strings.Contains(value, "查看提醒")
}

func reminderListText(values []domain.Reminder, location *time.Location) string {
	active := []domain.Reminder{}
	for _, value := range values {
		if value.Status == "active" || value.Status == "paused" {
			active = append(active, value)
		}
	}
	if len(active) == 0 {
		return "你目前没有有效提醒。"
	}
	sort.Slice(active, func(i, j int) bool {
		if active[i].NextFireAt == nil {
			return false
		}
		if active[j].NextFireAt == nil {
			return true
		}
		return active[i].NextFireAt.Before(*active[j].NextFireAt)
	})
	var builder strings.Builder
	builder.WriteString("你的提醒：")
	for index, value := range active {
		fmt.Fprintf(&builder, "\n%d. %s · %s", index+1, value.Content, describeSchedule(value.Schedule, location))
		if value.Status == "paused" {
			builder.WriteString("（已暂停）")
		}
	}
	return builder.String()
}

func previewReminderText(action domain.ReminderActionDraft, location *time.Location) string {
	if action.Action == "create" || action.Action == "update" {
		return fmt.Sprintf("请确认提醒设置：\n内容：%s\n时间：%s\n\n确认后才会生效。", action.Content, describeSchedule(*action.Schedule, location))
	}
	labels := map[string]string{"pause": "暂停", "resume": "恢复", "delete": "删除"}
	return fmt.Sprintf("请确认是否%s这个提醒。确认后操作才会生效。", labels[action.Action])
}

func confirmedReminderText(reminder domain.Reminder) string {
	if reminder.Status == "cancelled" {
		return "提醒已删除。"
	}
	if reminder.Status == "paused" {
		return "提醒已暂停。"
	}
	return "提醒已设置，我会通过飞书机器人私聊通知你。"
}

func describeSchedule(schedule domain.ReminderSchedule, location *time.Location) string {
	if schedule.Type == domain.ReminderOnce && schedule.OnceAt != nil {
		return schedule.OnceAt.In(location).Format("2006-01-02 15:04")
	}
	switch schedule.Type {
	case domain.ReminderDaily:
		return "每天 " + schedule.LocalTime
	case domain.ReminderWorkday:
		return "每个公司工作日 " + schedule.LocalTime
	case domain.ReminderWeekly:
		names := []string{"一", "二", "三", "四", "五", "六", "日"}
		parts := []string{}
		for _, weekday := range schedule.Weekdays {
			if weekday >= 1 && weekday <= 7 {
				parts = append(parts, "周"+names[weekday-1])
			}
		}
		return "每" + strings.Join(parts, "、") + " " + schedule.LocalTime
	default:
		return schedule.LocalTime
	}
}
