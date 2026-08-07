package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/store"
)

type MassageCardSender interface {
	Configured() bool
	SendMassageSignup(context.Context, string, string, time.Time, string, string, string) (string, error)
	SendMassageCall(context.Context, string, string, int, int, time.Time, string, string) (string, error)
	UpdateMassageCallCard(context.Context, string, string) error
}

type Massage struct {
	repo        store.MassageRepository
	base        store.Repository
	sender      MassageCardSender
	appLink     string
	responseTTL time.Duration
	retention   time.Duration
	now         func() time.Time
}

func NewMassage(repo store.MassageRepository, base store.Repository, sender MassageCardSender, appLink string, responseTTL, retention time.Duration) *Massage {
	if responseTTL <= 0 {
		responseTTL = 3 * time.Minute
	}
	if retention <= 0 {
		retention = 365 * 24 * time.Hour
	}
	return &Massage{repo: repo, base: base, sender: sender, appLink: appLink, responseTTL: responseTTL, retention: retention, now: time.Now}
}

func (m *Massage) CreateCycle(ctx context.Context, actor domain.User, value domain.MassageCycle) (domain.MassageCycle, error) {
	now := m.now()
	value.ID = ids.New("mcy")
	value.Status = domain.MassageCycleDraft
	value.CreatedBy = actor.ID
	value.CreatedAt = now
	value.UpdatedAt = now
	if value.Audience.Scope == "" {
		value.Audience.Scope = "all"
	}
	for i := range value.Sessions {
		value.Sessions[i].ID = ids.New("mss")
		value.Sessions[i].CycleID = value.ID
		value.Sessions[i].Sequence = i + 1
		value.Sessions[i].Status = "scheduled"
		value.Sessions[i].CreatedAt = now
		value.Sessions[i].UpdatedAt = now
	}
	if err := domain.ValidateMassageCycle(value); err != nil {
		return value, err
	}
	if err := m.repo.CreateMassageCycle(ctx, value); err != nil {
		return value, err
	}
	m.audit(ctx, actor, "massage.cycle.create", "massage_cycle", value.ID, map[string]any{"service_month": value.ServiceMonth})
	return m.repo.GetMassageCycle(ctx, value.ID)
}
func (m *Massage) UpdateCycle(ctx context.Context, actor domain.User, value domain.MassageCycle) (domain.MassageCycle, error) {
	old, err := m.repo.GetMassageCycle(ctx, value.ID)
	if err != nil {
		return value, err
	}
	value.Status = old.Status
	value.CreatedBy = old.CreatedBy
	value.CreatedAt = old.CreatedAt
	value.UpdatedAt = m.now()
	if err = domain.ValidateMassageCycle(value); err != nil {
		return value, err
	}
	var eligibleUserIDs []string
	var deliveries []domain.MassageDelivery
	if old.Status != domain.MassageCycleDraft {
		users, listErr := m.base.ListUsers(ctx)
		if listErr != nil {
			return value, listErr
		}
		for _, user := range users {
			if !value.Audience.Allows(user) {
				continue
			}
			eligibleUserIDs = append(eligibleUserIDs, user.ID)
			deliveries = append(deliveries, domain.MassageDelivery{
				ID: ids.New("mdl"), CycleID: value.ID, UserID: user.ID, Kind: "signup",
				Status: "pending", AvailableAt: value.SignupNoticeAt, IdempotencyKey: ids.New("idem"),
				CreatedAt: value.UpdatedAt, UpdatedAt: value.UpdatedAt,
			})
		}
		if len(eligibleUserIDs) == 0 {
			return value, fmt.Errorf("参与范围内没有可用的飞书员工")
		}
	}
	if err = m.repo.UpdateMassageCycle(ctx, value, eligibleUserIDs, deliveries); err != nil {
		return value, err
	}
	m.audit(ctx, actor, "massage.cycle.update", "massage_cycle", value.ID, nil)
	return m.repo.GetMassageCycle(ctx, value.ID)
}
func (m *Massage) ListCycles(ctx context.Context) ([]domain.MassageCycle, error) {
	return m.repo.ListMassageCycles(ctx)
}
func (m *Massage) GetCycle(ctx context.Context, id string) (domain.MassageCycle, error) {
	return m.repo.GetMassageCycle(ctx, id)
}
func (m *Massage) DeleteCycle(ctx context.Context, actor domain.User, id string) error {
	cycle, err := m.repo.GetMassageCycle(ctx, id)
	if err != nil {
		return err
	}
	if err = m.repo.DeleteMassageCycle(ctx, id); err != nil {
		return err
	}
	m.audit(ctx, actor, "massage.cycle.delete", "massage_cycle", id, map[string]any{"title": cycle.Title, "service_month": cycle.ServiceMonth, "status": cycle.Status})
	return nil
}
func (m *Massage) Publish(ctx context.Context, actor domain.User, id string) (domain.MassageCycle, error) {
	cycle, err := m.repo.GetMassageCycle(ctx, id)
	if err != nil {
		return cycle, err
	}
	users, err := m.base.ListUsers(ctx)
	if err != nil {
		return cycle, err
	}
	eligible := []string{}
	deliveries := []domain.MassageDelivery{}
	now := m.now()
	for _, user := range users {
		if !cycle.Audience.Allows(user) {
			continue
		}
		eligible = append(eligible, user.ID)
		deliveries = append(deliveries, domain.MassageDelivery{ID: ids.New("mdl"), CycleID: id, UserID: user.ID, Kind: "signup", Status: "pending", AvailableAt: cycle.SignupNoticeAt, IdempotencyKey: ids.New("idem"), CreatedAt: now, UpdatedAt: now})
	}
	if len(eligible) == 0 {
		return cycle, fmt.Errorf("参与范围内没有可用的飞书员工")
	}
	if err = m.repo.PublishMassageCycle(ctx, id, eligible, deliveries, now); err != nil {
		return cycle, err
	}
	m.audit(ctx, actor, "massage.cycle.publish", "massage_cycle", id, map[string]any{"eligible_count": len(eligible)})
	return m.repo.GetMassageCycle(ctx, id)
}
func (m *Massage) Respond(ctx context.Context, user domain.User, cycleID, action string) (domain.MassageEnrollment, error) {
	value, err := m.repo.MassageResponse(ctx, cycleID, user.ID, action, m.now())
	if err == nil {
		m.audit(ctx, user, "massage.enrollment."+action, "massage_cycle", cycleID, map[string]any{"queue_number": value.QueueNumber})
	}
	return value, err
}
func (m *Massage) MyCycles(ctx context.Context, user domain.User) ([]domain.MassageMe, error) {
	return m.repo.ListMassageMe(ctx, user.ID)
}
func (m *Massage) Resend(ctx context.Context, actor domain.User, cycleID string) (int, error) {
	idsList, err := m.repo.ListMassageEligibleUserIDs(ctx, cycleID)
	if err != nil {
		return 0, err
	}
	now := m.now()
	values := make([]domain.MassageDelivery, 0, len(idsList))
	for _, userID := range idsList {
		values = append(values, domain.MassageDelivery{ID: ids.New("mdl"), CycleID: cycleID, UserID: userID, Kind: "signup_resend", Status: "pending", AvailableAt: now, IdempotencyKey: ids.New("idem"), CreatedAt: now, UpdatedAt: now})
	}
	count, err := m.repo.CreateMassageResendDeliveries(ctx, cycleID, values)
	if err == nil {
		m.audit(ctx, actor, "massage.signup.resend", "massage_cycle", cycleID, map[string]any{"count": count})
	}
	return count, err
}
func (m *Massage) SessionAction(ctx context.Context, actor domain.User, id, action, reason string) (domain.MassageSession, error) {
	status := map[string]string{"start": "running", "resume": "running", "pause": "paused", "close": "closed"}[action]
	if status == "" {
		return domain.MassageSession{}, fmt.Errorf("invalid session action")
	}
	if action == "close" && strings.TrimSpace(reason) == "" {
		return domain.MassageSession{}, fmt.Errorf("结束未满额场次必须填写原因")
	}
	value, err := m.repo.SetMassageSessionStatus(ctx, id, status, strings.TrimSpace(reason), m.now())
	if err == nil {
		m.audit(ctx, actor, "massage.session."+action, "massage_session", id, map[string]any{"reason": reason})
	}
	return value, err
}
func (m *Massage) AdminCallAction(ctx context.Context, actor domain.User, id, action string) (domain.MassageCall, error) {
	value, err := m.repo.CompleteMassageCall(ctx, id, action, m.now())
	if err == nil {
		m.audit(ctx, actor, "massage.call."+action, "massage_call", id, map[string]any{"user_id": value.UserID})
	}
	return value, err
}
func (m *Massage) RespondCall(ctx context.Context, user domain.User, id, action string) (domain.MassageCall, error) {
	value, err := m.repo.RespondMassageCall(ctx, id, user.ID, action, m.now())
	if err == nil {
		m.audit(ctx, user, "massage.call."+action, "massage_call", id, nil)
	}
	return value, err
}
func (m *Massage) Statistics(ctx context.Context, id string) (domain.MassageStatistic, error) {
	return m.repo.MassageStatistics(ctx, id)
}
func (m *Massage) WriteCSV(ctx context.Context, id string, w io.Writer) error {
	stats, err := m.repo.MassageStatistics(ctx, id)
	if err != nil {
		return err
	}
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"排号", "姓名", "报名状态", "场次", "叫号时间", "响应状态", "完成时间"})
	calls := map[string][]domain.MassageCall{}
	for _, call := range stats.Calls {
		calls[call.EnrollmentID] = append(calls[call.EnrollmentID], call)
	}
	for _, e := range stats.Enrollments {
		values := calls[e.ID]
		if len(values) == 0 {
			_ = writer.Write([]string{strconv.Itoa(e.QueueNumber), e.UserName, e.Status, "", "", "", ""})
			continue
		}
		for _, call := range values {
			session := ""
			if cycle, cycleErr := m.repo.GetMassageCycle(ctx, id); cycleErr == nil {
				for _, s := range cycle.Sessions {
					if s.ID == call.SessionID {
						session = strconv.Itoa(s.Sequence)
					}
				}
			}
			_ = writer.Write([]string{strconv.Itoa(e.QueueNumber), e.UserName, e.Status, session, formatMassageTime(call.CalledAt), call.Status, formatMassageTime(call.CompletedAt)})
		}
	}
	writer.Flush()
	return writer.Error()
}
func formatMassageTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
}

func (m *Massage) HandleChat(ctx context.Context, user domain.User, conversationID, query, source string) (bool, domain.Message, error) {
	lower := strings.ToLower(strings.TrimSpace(query))
	if !LooksLikeMassage(lower) {
		return false, domain.Message{}, nil
	}
	items, err := m.repo.ListMassageMe(ctx, user.ID)
	if err != nil {
		return true, domain.Message{}, err
	}
	now := m.now()
	message := domain.Message{ID: ids.New("msg"), ConversationID: conversationID, Role: "assistant", Citations: []domain.Citation{}, CreatedAt: now}
	if len(items) == 0 {
		message.Content = "当前没有面向你的按摩报名批次。"
		return true, message, nil
	}
	if len(items) > 1 {
		message.Content = "当前有多个按摩批次，请在“我的按摩”页面选择具体月份。"
		return true, message, nil
	}
	item := items[0]
	cycle := item.Cycle
	if strings.Contains(lower, "退出") || strings.Contains(lower, "取消报名") {
		message.Content = "确认退出“" + cycle.Title + "”的按摩排号吗？退出后重新报名会排到队尾。"
		message.MassageAction = &domain.MassageAction{Type: "withdraw", CycleID: cycle.ID, Label: "确认退出"}
		return true, message, nil
	}
	if strings.Contains(lower, "报名") || strings.Contains(lower, "参加") || strings.Contains(lower, "排号") {
		if item.Enrollment != nil && item.Enrollment.Status == "enrolled" {
			message.Content = fmt.Sprintf("你已经报名“%s”，当前排号为 %d。", cycle.Title, item.Enrollment.QueueNumber)
			return true, message, nil
		}
		message.Content = "确认报名“" + cycle.Title + "”吗？排号将以确认成功时间为准。"
		message.MassageAction = &domain.MassageAction{Type: "enroll", CycleID: cycle.ID, Label: "确认报名"}
		return true, message, nil
	}
	if item.Enrollment == nil {
		message.Content = "你还没有报名“" + cycle.Title + "”。"
	} else {
		message.Content = fmt.Sprintf("你在“%s”中的排号是 %d，当前状态：%s。", cycle.Title, item.Enrollment.QueueNumber, item.Enrollment.Status)
	}
	return true, message, nil
}
func LooksLikeMassage(value string) bool {
	for _, word := range []string{"按摩", "排号", "叫号"} {
		if strings.Contains(value, word) {
			return true
		}
	}
	return false
}
func (m *Massage) audit(ctx context.Context, actor domain.User, action, resource, id string, metadata map[string]any) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	_ = m.base.AppendAudit(ctx, domain.AuditEvent{ID: ids.New("aud"), ActorID: actor.ID, ActorName: actor.Name, Action: action, ResourceType: resource, ResourceID: id, Metadata: metadata, CreatedAt: m.now()})
}

type MassageDispatcher struct {
	service   *Massage
	lastClean time.Time
}

func NewMassageDispatcher(service *Massage) *MassageDispatcher {
	return &MassageDispatcher{service: service}
}
func (d *MassageDispatcher) Tick(ctx context.Context) error {
	now := d.service.now()
	expired, err := d.service.repo.ExpireMassageCalls(ctx, now)
	if err != nil {
		return err
	}
	for _, call := range expired {
		if call.MessageID != "" && d.service.sender != nil {
			_ = d.service.sender.UpdateMassageCallCard(ctx, call.MessageID, "3分钟内未回复，已自动跳号")
		}
	}
	cycles, err := d.service.repo.ListMassageCycles(ctx)
	if err != nil {
		return err
	}
	for _, cycle := range cycles {
		for _, session := range cycle.Sessions {
			if session.Status == "running" {
				if _, err = d.service.repo.FillMassageSession(ctx, session.ID, now); err != nil {
					return err
				}
			}
		}
	}
	if err = d.deliver(ctx, now); err != nil {
		return err
	}
	if d.lastClean.IsZero() || now.Sub(d.lastClean) >= time.Hour {
		if err = d.service.repo.CleanupMassageHistory(ctx, now.Add(-d.service.retention)); err != nil {
			return err
		}
		d.lastClean = now
	}
	return nil
}
func (d *MassageDispatcher) deliver(ctx context.Context, now time.Time) error {
	values, err := d.service.repo.ClaimMassageDeliveries(ctx, now, 2*time.Minute, 50)
	if err != nil {
		return err
	}
	for _, delivery := range values {
		d.deliverOne(ctx, delivery)
	}
	return nil
}
func (d *MassageDispatcher) deliverOne(ctx context.Context, delivery domain.MassageDelivery) {
	now := d.service.now()
	user, err := d.service.base.GetUser(ctx, delivery.UserID)
	if err == nil && (user.Status != "active" || user.FeishuOpenID == "") {
		err = fmt.Errorf("用户不可用或缺少飞书 open_id")
	}
	if err == nil && (d.service.sender == nil || !d.service.sender.Configured()) {
		err = fmt.Errorf("飞书机器人未配置")
	}
	var messageID string
	if err == nil {
		cycle, getErr := d.service.repo.GetMassageCycle(ctx, delivery.CycleID)
		err = getErr
		if err == nil {
			if delivery.Kind == "call" {
				call, callErr := d.service.repo.GetMassageCall(ctx, delivery.CallID)
				err = callErr
				if err == nil {
					sequence := 0
					for _, session := range cycle.Sessions {
						if session.ID == delivery.SessionID {
							sequence = session.Sequence
						}
					}
					due := now.Add(d.service.responseTTL)
					messageID, err = d.service.sender.SendMassageCall(ctx, user.FeishuOpenID, cycle.Title, call.QueueNumber, sequence, due, call.ID, delivery.IdempotencyKey)
					if err == nil {
						err = d.service.repo.MarkMassageCallSent(ctx, call.ID, messageID, now, due)
					}
				}
			} else {
				messageID, err = d.service.sender.SendMassageSignup(ctx, user.FeishuOpenID, cycle.Title, cycle.SignupDeadline, cycle.ID, d.service.appLink, delivery.IdempotencyKey)
			}
		}
	}
	delivery.UpdatedAt = now
	delivery.LockedUntil = nil
	if err == nil {
		delivery.Status = "sent"
		delivery.MessageID = messageID
		delivery.LastError = ""
	} else if delivery.Attempts < 3 {
		delivery.Status = "retry"
		delivery.LastError = err.Error()
		delivery.AvailableAt = now.Add(time.Duration(delivery.Attempts) * time.Minute)
	} else {
		delivery.Status = "failed"
		delivery.LastError = err.Error()
		if delivery.CallID != "" {
			_ = d.service.repo.MarkMassageCallDeliveryFailed(ctx, delivery.CallID, err.Error(), now)
		}
	}
	_ = d.service.repo.UpdateMassageDelivery(ctx, delivery)
}
