package service

import (
	"context"
	"errors"
	"fmt"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/model"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/store"
	"sort"
	"strings"
	"time"
)

type Notification struct {
	repo    store.Repository
	feishu  *feishu.Client
	model   model.Provider
	scanner security.Scanner
}

func NewNotification(repo store.Repository, client *feishu.Client, provider model.Provider, scanners ...security.Scanner) *Notification {
	scanner := security.Scanner(security.NoopScanner{})
	if len(scanners) > 0 && scanners[0] != nil {
		scanner = scanners[0]
	}
	return &Notification{repo: repo, feishu: client, model: provider, scanner: scanner}
}

func (n *Notification) DraftWithAI(ctx context.Context, brief string) (string, error) {
	cfg, err := n.repo.PublishedConfig(ctx)
	if err != nil {
		return "", err
	}
	return n.model.Generate(ctx, model.GenerateRequest{Model: cfg.Config.GenerationModel, Temperature: .2, MaxTokens: 1000, Messages: []model.Message{{Role: "system", Content: "你是公司行政通知编辑。只润色用户给出的事实，不得增加日期、政策、收件范围或承诺。使用简洁正式的 Markdown 输出；不执行发送。"}, {Role: "user", Content: brief}}})
}

// Create retains the ACL-based behavior for drafts created by an older client.
func (n *Notification) Create(ctx context.Context, actor domain.User, title, content string, audience domain.ACL, scheduledAt *time.Time) (domain.NotificationDraft, error) {
	if err := validateNotificationContent(title, content, nil, scheduledAt); err != nil {
		return domain.NotificationDraft{}, err
	}
	if err := audience.Validate(); err != nil {
		return domain.NotificationDraft{}, err
	}
	users, _ := n.repo.ListUsers(ctx)
	count := 0
	for _, user := range users {
		if audience.Allows(user) {
			count++
		}
	}
	now := time.Now()
	value := domain.NotificationDraft{ID: ids.New("ntf"), Title: strings.TrimSpace(title), Content: strings.TrimSpace(content), ContentFormat: "markdown", Images: []domain.NotificationImage{}, RecipientType: "legacy", Recipients: []domain.NotificationRecipient{}, Audience: audience, Status: "draft", ScheduledAt: scheduledAt, CreatedBy: actor.ID, IdempotencyKey: ids.New("idem"), RecipientCount: count, CreatedAt: now, UpdatedAt: now}
	err := n.repo.CreateNotification(ctx, value)
	if err == nil {
		n.audit(ctx, actor, "notification.create", value.ID, map[string]any{"recipient_type": value.RecipientType, "recipient_count": count})
	}
	return value, err
}

func (n *Notification) CreateTargeted(ctx context.Context, actor domain.User, title, content, recipientType string, recipientIDs []string, images []domain.NotificationImage, scheduledAt *time.Time) (domain.NotificationDraft, error) {
	recipientType = strings.TrimSpace(recipientType)
	if recipientType != "user" && recipientType != "chat" {
		return domain.NotificationDraft{}, errors.New("recipient_type must be user or chat")
	}
	if err := validateNotificationContent(title, content, images, scheduledAt); err != nil {
		return domain.NotificationDraft{}, err
	}
	limit := 100
	if recipientType == "chat" {
		limit = 20
	}
	idsToResolve := uniqueNonEmpty(recipientIDs)
	if len(idsToResolve) == 0 {
		return domain.NotificationDraft{}, errors.New("at least one recipient is required")
	}
	if len(idsToResolve) > limit {
		return domain.NotificationDraft{}, fmt.Errorf("too many recipients; maximum is %d", limit)
	}
	recipients, err := n.resolveRecipients(ctx, recipientType, idsToResolve)
	if err != nil {
		return domain.NotificationDraft{}, err
	}
	now := time.Now()
	value := domain.NotificationDraft{
		ID: ids.New("ntf"), Title: strings.TrimSpace(title), Content: strings.TrimSpace(content), ContentFormat: "markdown", Images: images,
		RecipientType: recipientType, Recipients: recipients, Audience: domain.ACL{Scope: "restricted"}, Status: "draft", ScheduledAt: scheduledAt,
		CreatedBy: actor.ID, IdempotencyKey: ids.New("idem"), RecipientCount: len(recipients), CreatedAt: now, UpdatedAt: now,
	}
	if err = n.repo.CreateNotification(ctx, value); err == nil {
		n.audit(ctx, actor, "notification.create", value.ID, map[string]any{"recipient_type": recipientType, "recipient_count": len(recipients)})
	}
	return value, err
}

func (n *Notification) Targets(ctx context.Context) ([]domain.NotificationTargetOption, []domain.NotificationTargetOption, error) {
	users, err := n.repo.ListUsers(ctx)
	if err != nil {
		return nil, nil, err
	}
	userOptions := make([]domain.NotificationTargetOption, 0, len(users))
	for _, user := range users {
		if user.Status != "active" || user.FeishuOpenID == "" {
			continue
		}
		userOptions = append(userOptions, domain.NotificationTargetOption{Type: "user", ID: user.ID, Name: user.Name, AvatarURL: user.AvatarURL})
	}
	sort.Slice(userOptions, func(i, j int) bool { return userOptions[i].Name < userOptions[j].Name })
	if n.feishu == nil || !n.feishu.Configured() {
		return userOptions, []domain.NotificationTargetOption{}, errors.New("feishu is not configured; chat targets are unavailable")
	}
	chats, err := n.feishu.ListChats(ctx)
	if err != nil {
		return userOptions, []domain.NotificationTargetOption{}, err
	}
	chatOptions := make([]domain.NotificationTargetOption, 0, len(chats))
	for _, chat := range chats {
		chatOptions = append(chatOptions, domain.NotificationTargetOption{Type: "chat", ID: chat.ChatID, Name: chat.Name, AvatarURL: chat.AvatarURL})
	}
	sort.Slice(chatOptions, func(i, j int) bool { return chatOptions[i].Name < chatOptions[j].Name })
	return userOptions, chatOptions, nil
}

func (n *Notification) UploadImage(ctx context.Context, filename string, data []byte) (domain.NotificationImage, error) {
	if len(data) == 0 {
		return domain.NotificationImage{}, errors.New("image is empty")
	}
	if len(data) > 10*1024*1024 {
		return domain.NotificationImage{}, errors.New("image exceeds the 10 MB limit")
	}
	if err := n.scanner.Scan(ctx, data); err != nil {
		return domain.NotificationImage{}, fmt.Errorf("image security scan failed: %w", err)
	}
	if n.feishu == nil {
		return domain.NotificationImage{}, errors.New("feishu is not configured")
	}
	key, err := n.feishu.UploadMessageImage(ctx, filename, data)
	if err != nil {
		return domain.NotificationImage{}, err
	}
	name := strings.TrimSpace(filename)
	if name == "" {
		name = "通知图片"
	}
	return domain.NotificationImage{ImageKey: key, Name: name, Alt: name}, nil
}

func (n *Notification) Approve(ctx context.Context, actor domain.User, id string) (domain.NotificationDraft, error) {
	value, err := n.repo.GetNotification(ctx, id)
	if err != nil {
		return value, err
	}
	if value.Status != "draft" {
		return value, errors.New("only draft notifications can be approved")
	}
	value.ApprovedBy = actor.ID
	value.Status = "approved"
	if value.ScheduledAt != nil && value.ScheduledAt.After(time.Now()) {
		value.Status = "scheduled"
	}
	value.LastError = ""
	value.UpdatedAt = time.Now()
	if err = n.repo.UpdateNotification(ctx, value); err == nil {
		n.audit(ctx, actor, "notification.approve", id, map[string]any{"status": value.Status})
	}
	return value, err
}

func (n *Notification) Send(ctx context.Context, actor domain.User, id string) (domain.NotificationDraft, error) {
	value, err := n.repo.GetNotification(ctx, id)
	if err != nil {
		return value, err
	}
	if value.Status != "approved" && value.Status != "scheduled" && value.Status != "failed" && value.Status != "partial" {
		return value, errors.New("notification requires approval")
	}
	value.Status = "sending"
	value.UpdatedAt = time.Now()
	if err = n.repo.UpdateNotification(ctx, value); err != nil {
		return value, err
	}
	value = n.deliver(ctx, value)
	n.audit(ctx, actor, "notification.send", id, map[string]any{"status": value.Status, "last_error": value.LastError})
	return value, nil
}

func (n *Notification) DispatchDue(ctx context.Context) error {
	values, err := n.repo.ClaimDueNotifications(ctx, time.Now(), 20)
	if err != nil {
		return err
	}
	for _, value := range values {
		value = n.deliver(ctx, value)
		n.audit(ctx, domain.User{Name: "系统"}, "notification.send_scheduled", value.ID, map[string]any{"status": value.Status, "last_error": value.LastError})
	}
	return nil
}

func (n *Notification) deliver(ctx context.Context, value domain.NotificationDraft) domain.NotificationDraft {
	images := make([]feishu.CardImage, 0, len(value.Images))
	for _, image := range value.Images {
		images = append(images, feishu.CardImage{ImageKey: image.ImageKey, Alt: image.Alt})
	}
	successes, failures := 0, []string{}
	if len(value.Recipients) > 0 {
		for _, recipient := range value.Recipients {
			receiveType, receiveID, resolveErr := n.deliveryAddress(ctx, recipient)
			if resolveErr != nil {
				failures = append(failures, recipient.Name+": "+resolveErr.Error())
				_ = n.repo.RecordNotificationDelivery(ctx, value.ID, recipient.ID, recipient.Type, recipient.Name, "failed", "", resolveErr.Error(), 1, time.Now())
				continue
			}
			messageID := "demo"
			var sendErr error
			if n.feishu != nil && n.feishu.Configured() {
				messageID, sendErr = n.feishu.SendRichCard(ctx, receiveType, receiveID, value.Title, value.Content, images, value.IdempotencyKey+"_"+recipient.ID)
			}
			if sendErr != nil {
				failures = append(failures, recipient.Name+": "+sendErr.Error())
				_ = n.repo.RecordNotificationDelivery(ctx, value.ID, recipient.ID, recipient.Type, recipient.Name, "failed", "", sendErr.Error(), 1, time.Now())
				continue
			}
			successes++
			_ = n.repo.RecordNotificationDelivery(ctx, value.ID, recipient.ID, recipient.Type, recipient.Name, "sent", messageID, "", 1, time.Now())
		}
	} else {
		users, _ := n.repo.ListUsers(ctx)
		for _, user := range users {
			if !value.Audience.Allows(user) || user.FeishuOpenID == "" {
				continue
			}
			messageID := "demo"
			var sendErr error
			if n.feishu != nil && n.feishu.Configured() {
				messageID, sendErr = n.feishu.SendRichCard(ctx, "open_id", user.FeishuOpenID, value.Title, value.Content, images, value.IdempotencyKey+"_"+user.ID)
			}
			if sendErr != nil {
				failures = append(failures, user.Name+": "+sendErr.Error())
				_ = n.repo.RecordNotificationDelivery(ctx, value.ID, user.ID, "user", user.Name, "failed", "", sendErr.Error(), 1, time.Now())
				continue
			}
			successes++
			_ = n.repo.RecordNotificationDelivery(ctx, value.ID, user.ID, "user", user.Name, "sent", messageID, "", 1, time.Now())
		}
	}
	value.Status = "sent"
	if successes == 0 && len(failures) > 0 {
		value.Status = "failed"
	} else if len(failures) > 0 {
		value.Status = "partial"
	}
	value.LastError = truncate(strings.Join(failures, "; "), 2000)
	value.UpdatedAt = time.Now()
	_ = n.repo.UpdateNotification(ctx, value)
	return value
}

func (n *Notification) Cancel(ctx context.Context, actor domain.User, id string) (domain.NotificationDraft, error) {
	value, err := n.repo.GetNotification(ctx, id)
	if err != nil {
		return value, err
	}
	if value.Status == "sent" || value.Status == "sending" {
		return value, fmt.Errorf("%s notification cannot be cancelled", value.Status)
	}
	value.Status = "cancelled"
	value.UpdatedAt = time.Now()
	err = n.repo.UpdateNotification(ctx, value)
	if err == nil {
		n.audit(ctx, actor, "notification.cancel", id, nil)
	}
	return value, err
}

func (n *Notification) resolveRecipients(ctx context.Context, recipientType string, recipientIDs []string) ([]domain.NotificationRecipient, error) {
	requested := make(map[string]bool, len(recipientIDs))
	for _, id := range recipientIDs {
		requested[id] = true
	}
	resolved := make([]domain.NotificationRecipient, 0, len(requested))
	if recipientType == "user" {
		users, err := n.repo.ListUsers(ctx)
		if err != nil {
			return nil, err
		}
		for _, user := range users {
			if requested[user.ID] && user.Status == "active" && user.FeishuOpenID != "" {
				resolved = append(resolved, domain.NotificationRecipient{Type: "user", ID: user.ID, Name: user.Name})
			}
		}
	} else {
		if n.feishu == nil {
			return nil, errors.New("feishu is not configured")
		}
		chats, err := n.feishu.ListChats(ctx)
		if err != nil {
			return nil, err
		}
		for _, chat := range chats {
			if requested[chat.ChatID] {
				resolved = append(resolved, domain.NotificationRecipient{Type: "chat", ID: chat.ChatID, Name: chat.Name})
			}
		}
	}
	if len(resolved) != len(requested) {
		return nil, errors.New("one or more recipients are unavailable or the bot is no longer in the selected chat")
	}
	return resolved, nil
}

func (n *Notification) deliveryAddress(ctx context.Context, recipient domain.NotificationRecipient) (string, string, error) {
	switch recipient.Type {
	case "chat":
		if recipient.ID == "" {
			return "", "", errors.New("chat_id is empty")
		}
		return "chat_id", recipient.ID, nil
	case "user":
		user, err := n.repo.GetUser(ctx, recipient.ID)
		if err != nil {
			return "", "", err
		}
		if user.Status != "active" || user.FeishuOpenID == "" {
			return "", "", errors.New("user is inactive or has no Feishu identity")
		}
		return "open_id", user.FeishuOpenID, nil
	default:
		return "", "", errors.New("unsupported recipient type")
	}
}

func validateNotificationContent(title, content string, images []domain.NotificationImage, scheduledAt *time.Time) error {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		return errors.New("title and content are required")
	}
	if len([]rune(strings.TrimSpace(title))) > 80 || len([]rune(strings.TrimSpace(content))) > 4000 {
		return errors.New("title or content exceeds the length limit")
	}
	if len(images) > 9 {
		return errors.New("a notification can contain at most 9 images")
	}
	for _, image := range images {
		if strings.TrimSpace(image.ImageKey) == "" {
			return errors.New("image_key is required")
		}
	}
	if scheduledAt != nil && scheduledAt.Before(time.Now().Add(-30*time.Second)) {
		return errors.New("scheduled_at cannot be in the past")
	}
	return nil
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func (n *Notification) audit(ctx context.Context, actor domain.User, action, id string, metadata map[string]any) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	_ = n.repo.AppendAudit(ctx, domain.AuditEvent{ID: ids.New("aud"), ActorID: actor.ID, ActorName: actor.Name, Action: action, ResourceType: "notification", ResourceID: id, Metadata: metadata, CreatedAt: time.Now()})
}
