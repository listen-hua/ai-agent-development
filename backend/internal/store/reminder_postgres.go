package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
)

func (p *Postgres) CreateReminderAction(ctx context.Context, value domain.ReminderActionDraft) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO reminder_action_drafts(id,user_id,action,reminder_id,content,schedule,status,source_channel,source_conversation_id,result_reminder_id,expires_at,created_at,confirmed_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NULL,$10,$11,NULL)`, value.ID, value.UserID, value.Action, nullUUID(value.ReminderID), value.Content, nullableJSON(value.Schedule), value.Status, value.SourceChannel, nullUUID(value.SourceConversationID), value.ExpiresAt, value.CreatedAt)
	return err
}

func (p *Postgres) CountReminderActionsSince(ctx context.Context, userID string, since time.Time) (int, error) {
	var count int
	err := p.pool.QueryRow(ctx, `SELECT count(*) FROM reminder_action_drafts WHERE user_id=$1 AND created_at>=$2`, userID, since).Scan(&count)
	return count, err
}

func nullableJSON(value any) any {
	if value == nil {
		return nil
	}
	return mustJSON(value)
}

func (p *Postgres) GetReminderAction(ctx context.Context, id string) (domain.ReminderActionDraft, error) {
	return scanReminderAction(p.pool.QueryRow(ctx, reminderActionSelect+` WHERE a.id=$1`, id))
}

func (p *Postgres) LatestPendingReminderAction(ctx context.Context, userID, conversationID string) (domain.ReminderActionDraft, error) {
	query := reminderActionSelect + ` WHERE a.user_id=$1 AND a.status='pending'`
	args := []any{userID}
	if conversationID != "" {
		query += ` AND a.source_conversation_id=$2`
		args = append(args, conversationID)
	}
	query += ` ORDER BY a.created_at DESC LIMIT 1`
	return scanReminderAction(p.pool.QueryRow(ctx, query, args...))
}

const reminderActionSelect = `SELECT a.id,a.user_id,a.action,COALESCE(a.reminder_id::text,''),a.content,a.schedule,a.status,a.source_channel,COALESCE(a.source_conversation_id::text,''),COALESCE(a.result_reminder_id::text,''),a.expires_at,a.created_at,a.confirmed_at FROM reminder_action_drafts a`

type rowScanner interface{ Scan(...any) error }

func scanReminderAction(row rowScanner) (domain.ReminderActionDraft, error) {
	var value domain.ReminderActionDraft
	var raw []byte
	if err := row.Scan(&value.ID, &value.UserID, &value.Action, &value.ReminderID, &value.Content, &raw, &value.Status, &value.SourceChannel, &value.SourceConversationID, &value.ResultReminderID, &value.ExpiresAt, &value.CreatedAt, &value.ConfirmedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return value, ErrNotFound
		}
		return value, err
	}
	if len(raw) > 0 && string(raw) != "null" {
		var schedule domain.ReminderSchedule
		if err := json.Unmarshal(raw, &schedule); err != nil {
			return value, err
		}
		value.Schedule = &schedule
	}
	return value, nil
}

func (p *Postgres) ConfirmReminderAction(ctx context.Context, id, userID string, nextFireAt *time.Time, now time.Time) (domain.ReminderActionDraft, domain.Reminder, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.ReminderActionDraft{}, domain.Reminder{}, err
	}
	defer tx.Rollback(ctx)
	action, err := scanReminderAction(tx.QueryRow(ctx, reminderActionSelect+` WHERE a.id=$1 FOR UPDATE`, id))
	if err != nil {
		return action, domain.Reminder{}, err
	}
	if action.UserID != userID {
		return action, domain.Reminder{}, ErrForbidden
	}
	if action.Status == "confirmed" && action.ResultReminderID != "" {
		reminder, getErr := getReminderWith(ctx, tx, action.ResultReminderID)
		return action, reminder, getErr
	}
	if action.Status != "pending" || now.After(action.ExpiresAt) {
		if action.Status == "pending" {
			_, _ = tx.Exec(ctx, `UPDATE reminder_action_drafts SET status='expired' WHERE id=$1`, id)
			_ = tx.Commit(ctx)
		}
		return action, domain.Reminder{}, ErrConflict
	}
	var reminder domain.Reminder
	switch action.Action {
	case "create":
		if action.Schedule == nil {
			return action, reminder, ErrConflict
		}
		reminder = domain.Reminder{ID: ids.New("rem"), UserID: userID, Content: action.Content, Status: "active", Schedule: *action.Schedule, NextFireAt: nextFireAt, SourceChannel: action.SourceChannel, SourceConversationID: action.SourceConversationID, Version: 1, CreatedAt: now, UpdatedAt: now}
		if err = insertReminder(ctx, tx, reminder); err != nil {
			return action, reminder, err
		}
	case "update", "pause", "resume", "delete":
		reminder, err = getReminderWith(ctx, tx, action.ReminderID)
		if err != nil {
			return action, reminder, err
		}
		if reminder.UserID != userID {
			return action, reminder, ErrForbidden
		}
		if action.Action == "update" {
			if action.Content != "" {
				reminder.Content = action.Content
			}
			if action.Schedule != nil {
				reminder.Schedule = *action.Schedule
			}
			reminder.Status, reminder.NextFireAt = "active", nextFireAt
		} else if action.Action == "pause" {
			reminder.Status, reminder.NextFireAt = "paused", nil
		} else if action.Action == "resume" {
			reminder.Status, reminder.NextFireAt = "active", nextFireAt
		} else {
			reminder.Status, reminder.NextFireAt = "cancelled", nil
		}
		reminder.Version++
		reminder.UpdatedAt = now
		if err = updateReminder(ctx, tx, reminder); err != nil {
			return action, reminder, err
		}
	default:
		return action, reminder, ErrConflict
	}
	confirmed := now
	action.Status, action.ResultReminderID, action.ConfirmedAt = "confirmed", reminder.ID, &confirmed
	if _, err = tx.Exec(ctx, `UPDATE reminder_action_drafts SET status='confirmed',result_reminder_id=$2,confirmed_at=$3 WHERE id=$1`, action.ID, reminder.ID, now); err != nil {
		return action, reminder, err
	}
	if err = tx.Commit(ctx); err != nil {
		return action, reminder, err
	}
	return action, reminder, nil
}

func (p *Postgres) CancelReminderAction(ctx context.Context, id, userID string, now time.Time) (domain.ReminderActionDraft, error) {
	action, err := p.GetReminderAction(ctx, id)
	if err != nil {
		return action, err
	}
	if action.UserID != userID {
		return action, ErrForbidden
	}
	if action.Status == "cancelled" {
		return action, nil
	}
	if action.Status != "pending" {
		return action, ErrConflict
	}
	tag, err := p.pool.Exec(ctx, `UPDATE reminder_action_drafts SET status='cancelled',confirmed_at=$3 WHERE id=$1 AND user_id=$2 AND status='pending'`, id, userID, now)
	if err != nil {
		return action, err
	}
	if tag.RowsAffected() == 0 {
		return action, ErrConflict
	}
	action.Status, action.ConfirmedAt = "cancelled", &now
	return action, nil
}

func insertReminder(ctx context.Context, tx pgx.Tx, value domain.Reminder) error {
	localTime, onceAt := reminderScheduleValues(value.Schedule)
	weekdays := value.Schedule.Weekdays
	if weekdays == nil {
		weekdays = []int{}
	}
	_, err := tx.Exec(ctx, `INSERT INTO reminders(id,user_id,content,status,schedule_type,timezone,local_time,weekdays,once_at,next_fire_at,last_fired_at,last_error,source_channel,source_conversation_id,version,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`, value.ID, value.UserID, value.Content, value.Status, value.Schedule.Type, value.Schedule.Timezone, localTime, weekdays, onceAt, value.NextFireAt, value.LastFiredAt, value.LastError, value.SourceChannel, nullUUID(value.SourceConversationID), value.Version, value.CreatedAt, value.UpdatedAt)
	return err
}

func updateReminder(ctx context.Context, tx pgx.Tx, value domain.Reminder) error {
	localTime, onceAt := reminderScheduleValues(value.Schedule)
	weekdays := value.Schedule.Weekdays
	if weekdays == nil {
		weekdays = []int{}
	}
	_, err := tx.Exec(ctx, `UPDATE reminders SET content=$2,status=$3,schedule_type=$4,timezone=$5,local_time=$6,weekdays=$7,once_at=$8,next_fire_at=$9,last_error=$10,version=$11,updated_at=$12 WHERE id=$1`, value.ID, value.Content, value.Status, value.Schedule.Type, value.Schedule.Timezone, localTime, weekdays, onceAt, value.NextFireAt, value.LastError, value.Version, value.UpdatedAt)
	return err
}

func reminderScheduleValues(schedule domain.ReminderSchedule) (any, any) {
	var localTime, onceAt any
	if schedule.LocalTime != "" {
		localTime = schedule.LocalTime
	}
	if schedule.OnceAt != nil {
		onceAt = schedule.OnceAt
	}
	return localTime, onceAt
}

const reminderSelect = `SELECT r.id,r.user_id,r.content,r.status,r.schedule_type,r.timezone,COALESCE(to_char(r.local_time,'HH24:MI'),''),r.weekdays,r.once_at,r.next_fire_at,r.last_fired_at,r.last_error,r.source_channel,COALESCE(r.source_conversation_id::text,''),r.version,r.created_at,r.updated_at FROM reminders r`

func getReminderWith(ctx context.Context, queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (domain.Reminder, error) {
	return scanReminder(queryer.QueryRow(ctx, reminderSelect+` WHERE r.id=$1`, id))
}

func scanReminder(row rowScanner) (domain.Reminder, error) {
	var value domain.Reminder
	if err := row.Scan(&value.ID, &value.UserID, &value.Content, &value.Status, &value.Schedule.Type, &value.Schedule.Timezone, &value.Schedule.LocalTime, &value.Schedule.Weekdays, &value.Schedule.OnceAt, &value.NextFireAt, &value.LastFiredAt, &value.LastError, &value.SourceChannel, &value.SourceConversationID, &value.Version, &value.CreatedAt, &value.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return value, ErrNotFound
		}
		return value, err
	}
	return value, nil
}

func (p *Postgres) ListReminders(ctx context.Context, userID string) ([]domain.Reminder, error) {
	rows, err := p.pool.Query(ctx, reminderSelect+` WHERE r.user_id=$1 ORDER BY r.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Reminder{}
	for rows.Next() {
		value, scanErr := scanReminder(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

func (p *Postgres) GetReminder(ctx context.Context, id string) (domain.Reminder, error) {
	return getReminderWith(ctx, p.pool, id)
}

func (p *Postgres) ListReminderDeliveries(ctx context.Context, reminderID string, limit int) ([]domain.ReminderDelivery, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := p.pool.Query(ctx, deliverySelect+` WHERE d.reminder_id=$1 ORDER BY d.scheduled_for DESC LIMIT $2`, reminderID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ReminderDelivery{}
	for rows.Next() {
		value, scanErr := scanDelivery(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

func (p *Postgres) ListDueReminders(ctx context.Context, now time.Time, limit int) ([]domain.Reminder, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := p.pool.Query(ctx, reminderSelect+` WHERE r.status='active' AND r.next_fire_at<=$1 ORDER BY r.next_fire_at LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Reminder{}
	for rows.Next() {
		value, scanErr := scanReminder(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

func (p *Postgres) ListActiveWorkdayReminders(ctx context.Context) ([]domain.Reminder, error) {
	rows, err := p.pool.Query(ctx, reminderSelect+` WHERE r.status='active' AND r.schedule_type='workday' ORDER BY r.next_fire_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]domain.Reminder, 0)
	for rows.Next() {
		value, scanErr := scanReminder(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (p *Postgres) CreateReminderDeliveryAndAdvance(ctx context.Context, reminderID string, scheduledFor time.Time, nextFireAt *time.Time, now time.Time) (domain.ReminderDelivery, bool, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.ReminderDelivery{}, false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE reminders SET next_fire_at=$3,updated_at=$4 WHERE id=$1 AND status='active' AND next_fire_at=$2`, reminderID, scheduledFor, nextFireAt, now)
	if err != nil {
		return domain.ReminderDelivery{}, false, err
	}
	if tag.RowsAffected() == 0 {
		return domain.ReminderDelivery{}, false, nil
	}
	delivery := domain.ReminderDelivery{ID: ids.New("del"), ReminderID: reminderID, ScheduledFor: scheduledFor, Status: "pending", NextAttemptAt: now, IdempotencyKey: ids.New("idem"), CreatedAt: now, UpdatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO reminder_deliveries(id,reminder_id,scheduled_for,status,attempts,next_attempt_at,idempotency_key,created_at,updated_at) VALUES($1,$2,$3,$4,0,$5,$6,$7,$8) ON CONFLICT(reminder_id,scheduled_for) DO NOTHING`, delivery.ID, delivery.ReminderID, delivery.ScheduledFor, delivery.Status, delivery.NextAttemptAt, delivery.IdempotencyKey, delivery.CreatedAt, delivery.UpdatedAt)
	if err != nil {
		return delivery, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return delivery, false, err
	}
	return delivery, true, nil
}

const deliverySelect = `SELECT d.id,d.reminder_id,d.scheduled_for,d.status,d.attempts,d.next_attempt_at,d.locked_until,d.message_id,d.error,d.idempotency_key,d.created_at,d.updated_at FROM reminder_deliveries d`

func scanDelivery(row rowScanner) (domain.ReminderDelivery, error) {
	var value domain.ReminderDelivery
	err := row.Scan(&value.ID, &value.ReminderID, &value.ScheduledFor, &value.Status, &value.Attempts, &value.NextAttemptAt, &value.LockedUntil, &value.MessageID, &value.Error, &value.IdempotencyKey, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func (p *Postgres) ClaimReminderDeliveries(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.ReminderDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := p.pool.Query(ctx, `WITH picked AS (
		SELECT id FROM reminder_deliveries WHERE status IN ('pending','retry','sending') AND next_attempt_at<=$1 AND (locked_until IS NULL OR locked_until<$1) ORDER BY next_attempt_at FOR UPDATE SKIP LOCKED LIMIT $2
	) UPDATE reminder_deliveries d SET status='sending',attempts=d.attempts+1,locked_until=$1+$3::interval,updated_at=$1 FROM picked WHERE d.id=picked.id
	RETURNING d.id,d.reminder_id,d.scheduled_for,d.status,d.attempts,d.next_attempt_at,d.locked_until,d.message_id,d.error,d.idempotency_key,d.created_at,d.updated_at`, now, limit, fmt.Sprintf("%f seconds", lease.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ReminderDelivery{}
	for rows.Next() {
		value, scanErr := scanDelivery(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

func (p *Postgres) UpdateReminderDelivery(ctx context.Context, value domain.ReminderDelivery) error {
	_, err := p.pool.Exec(ctx, `UPDATE reminder_deliveries SET status=$2,attempts=$3,next_attempt_at=$4,locked_until=$5,message_id=$6,error=$7,updated_at=$8 WHERE id=$1`, value.ID, value.Status, value.Attempts, value.NextAttemptAt, value.LockedUntil, value.MessageID, value.Error, value.UpdatedAt)
	return err
}

func (p *Postgres) UpdateReminderRuntime(ctx context.Context, id, status string, lastFiredAt *time.Time, lastError string, now time.Time) error {
	_, err := p.pool.Exec(ctx, `UPDATE reminders SET status=CASE WHEN $2='' THEN status ELSE $2 END,last_fired_at=COALESCE($3,last_fired_at),last_error=$4,updated_at=$5 WHERE id=$1`, id, status, lastFiredAt, lastError, now)
	return err
}

func (p *Postgres) UpdateReminderNextFire(ctx context.Context, id string, nextFireAt *time.Time, now time.Time) error {
	tag, err := p.pool.Exec(ctx, `UPDATE reminders SET next_fire_at=$2,updated_at=$3 WHERE id=$1 AND status='active'`, id, nextFireAt, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) CleanupReminderHistory(ctx context.Context, before time.Time) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM reminders WHERE status IN ('completed','cancelled','failed') AND updated_at<$1`, before); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM reminder_action_drafts WHERE status<>'pending' AND created_at<$1`, before); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) ListWorkdayOverrides(ctx context.Context, year int) ([]domain.WorkdayOverride, error) {
	rows, err := p.pool.Query(ctx, `SELECT to_char(work_date,'YYYY-MM-DD'),is_workday,note,COALESCE(updated_by::text,''),updated_at FROM workday_overrides WHERE EXTRACT(YEAR FROM work_date)=$1 ORDER BY work_date`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.WorkdayOverride{}
	for rows.Next() {
		var value domain.WorkdayOverride
		if err = rows.Scan(&value.Date, &value.IsWorkday, &value.Note, &value.UpdatedBy, &value.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

func (p *Postgres) GetWorkdayOverride(ctx context.Context, date string) (domain.WorkdayOverride, error) {
	var value domain.WorkdayOverride
	err := p.pool.QueryRow(ctx, `SELECT to_char(work_date,'YYYY-MM-DD'),is_workday,note,COALESCE(updated_by::text,''),updated_at FROM workday_overrides WHERE work_date=$1`, date).Scan(&value.Date, &value.IsWorkday, &value.Note, &value.UpdatedBy, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	return value, err
}

func (p *Postgres) UpsertWorkdayOverride(ctx context.Context, value domain.WorkdayOverride) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO workday_overrides(work_date,is_workday,note,updated_by,updated_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(work_date) DO UPDATE SET is_workday=excluded.is_workday,note=excluded.note,updated_by=excluded.updated_by,updated_at=excluded.updated_at`, value.Date, value.IsWorkday, value.Note, nullUUID(value.UpdatedBy), value.UpdatedAt)
	return err
}

func (p *Postgres) DeleteWorkdayOverride(ctx context.Context, date string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM workday_overrides WHERE work_date=$1`, date)
	return err
}

func (p *Postgres) CreateReminderBotJob(ctx context.Context, value domain.ReminderBotJob) (bool, error) {
	tag, err := p.pool.Exec(ctx, `INSERT INTO reminder_bot_jobs(id,event_id,open_id,chat_id,message_id,content,status,attempts,available_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(event_id) DO NOTHING`, value.ID, value.EventID, value.OpenID, value.ChatID, value.MessageID, value.Content, value.Status, value.Attempts, value.AvailableAt, value.CreatedAt, value.UpdatedAt)
	return err == nil && tag.RowsAffected() > 0, err
}

func (p *Postgres) ClaimReminderBotJobs(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.ReminderBotJob, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := p.pool.Query(ctx, `WITH picked AS (
		SELECT id FROM reminder_bot_jobs WHERE status IN ('pending','retry','processing') AND available_at<=$1 AND (locked_until IS NULL OR locked_until<$1) ORDER BY available_at FOR UPDATE SKIP LOCKED LIMIT $2
	) UPDATE reminder_bot_jobs j SET status='processing',attempts=j.attempts+1,locked_until=$1+$3::interval,updated_at=$1 FROM picked WHERE j.id=picked.id
	RETURNING j.id,j.event_id,j.open_id,j.chat_id,j.message_id,j.content,j.status,j.attempts,j.last_error,j.available_at,j.created_at,j.updated_at`, now, limit, fmt.Sprintf("%f seconds", lease.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.ReminderBotJob{}
	for rows.Next() {
		var value domain.ReminderBotJob
		if err = rows.Scan(&value.ID, &value.EventID, &value.OpenID, &value.ChatID, &value.MessageID, &value.Content, &value.Status, &value.Attempts, &value.LastError, &value.AvailableAt, &value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

func (p *Postgres) UpdateReminderBotJob(ctx context.Context, value domain.ReminderBotJob) error {
	_, err := p.pool.Exec(ctx, `UPDATE reminder_bot_jobs SET status=$2,attempts=$3,last_error=$4,available_at=$5,locked_until=NULL,updated_at=$6 WHERE id=$1`, value.ID, value.Status, value.Attempts, value.LastError, value.AvailableAt, value.UpdatedAt)
	return err
}
