package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
)

const massageCycleSelect = `SELECT c.id,c.service_month,c.title,c.signup_notice_at,c.signup_deadline,c.audience,c.status,c.next_queue,
 c.created_by,c.created_at,c.updated_at,
 (SELECT count(*) FROM massage_eligible_users e WHERE e.cycle_id=c.id),
 (SELECT count(*) FROM massage_enrollments n WHERE n.cycle_id=c.id AND n.status='enrolled')
 FROM massage_cycles c `

func (p *Postgres) ListMassageCycles(ctx context.Context) ([]domain.MassageCycle, error) {
	rows, err := p.pool.Query(ctx, massageCycleSelect+`ORDER BY c.service_month DESC,c.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.MassageCycle{}
	for rows.Next() {
		value, scanErr := scanMassageCycle(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		value.Sessions, scanErr = p.listMassageSessions(ctx, value.ID)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (p *Postgres) GetMassageCycle(ctx context.Context, id string) (domain.MassageCycle, error) {
	value, err := scanMassageCycle(p.pool.QueryRow(ctx, massageCycleSelect+`WHERE c.id=$1`, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			err = ErrNotFound
		}
		return value, err
	}
	value.Sessions, err = p.listMassageSessions(ctx, id)
	return value, err
}

type massageScanner interface{ Scan(...any) error }

func scanMassageCycle(row massageScanner) (domain.MassageCycle, error) {
	var value domain.MassageCycle
	var raw []byte
	err := row.Scan(&value.ID, &value.ServiceMonth, &value.Title, &value.SignupNoticeAt, &value.SignupDeadline, &raw, &value.Status, &value.NextQueue, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt, &value.EligibleCount, &value.EnrolledCount)
	if err == nil {
		_ = json.Unmarshal(raw, &value.Audience)
	}
	return value, err
}

func (p *Postgres) listMassageSessions(ctx context.Context, cycleID string) ([]domain.MassageSession, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,cycle_id,sequence,starts_at,quota,concurrent_slots,status,completed_count,started_at,closed_at,close_reason,created_at,updated_at FROM massage_sessions WHERE cycle_id=$1 ORDER BY sequence`, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.MassageSession{}
	for rows.Next() {
		value, scanErr := scanMassageSession(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func scanMassageSession(row massageScanner) (domain.MassageSession, error) {
	var value domain.MassageSession
	err := row.Scan(&value.ID, &value.CycleID, &value.Sequence, &value.StartsAt, &value.Quota, &value.ConcurrentSlots, &value.Status, &value.CompletedCount, &value.StartedAt, &value.ClosedAt, &value.CloseReason, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func (p *Postgres) CreateMassageCycle(ctx context.Context, value domain.MassageCycle) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO massage_cycles(id,service_month,title,signup_notice_at,signup_deadline,audience,status,next_queue,created_by,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,0,$8,$9,$9)`, value.ID, value.ServiceMonth, value.Title, value.SignupNoticeAt, value.SignupDeadline, mustJSON(value.Audience), value.Status, value.CreatedBy, value.CreatedAt)
	if err != nil {
		return err
	}
	for _, session := range value.Sessions {
		_, err = tx.Exec(ctx, `INSERT INTO massage_sessions(id,cycle_id,sequence,starts_at,quota,concurrent_slots,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,'scheduled',$7,$7)`, session.ID, value.ID, session.Sequence, session.StartsAt, session.Quota, session.ConcurrentSlots, value.CreatedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) UpdateMassageCycle(ctx context.Context, value domain.MassageCycle, eligibleUserIDs []string, deliveries []domain.MassageDelivery) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var tag pgconn.CommandTag
	tag, err = tx.Exec(ctx, `UPDATE massage_cycles SET service_month=$2,title=$3,signup_notice_at=$4,signup_deadline=$5,audience=$6,updated_at=$7 WHERE id=$1 AND status NOT IN ('completed','cancelled')`, value.ID, value.ServiceMonth, value.Title, value.SignupNoticeAt, value.SignupDeadline, mustJSON(value.Audience), value.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	for _, session := range value.Sessions {
		tag, err = tx.Exec(ctx, `UPDATE massage_sessions s SET starts_at=$3,quota=$4,concurrent_slots=$5,updated_at=$6
		 WHERE s.id=$1 AND s.cycle_id=$2
		 AND $4 >= s.completed_count + (SELECT count(*) FROM massage_calls c WHERE c.session_id=s.id AND c.status IN ('pending_delivery','awaiting_response','accepted'))`, session.ID, value.ID, session.StartsAt, session.Quota, session.ConcurrentSlots, value.UpdatedAt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrConflict
		}
	}
	if value.Status != domain.MassageCycleDraft {
		if _, err = tx.Exec(ctx, `DELETE FROM massage_eligible_users e WHERE e.cycle_id=$1 AND NOT (e.user_id = ANY($2::text[])) AND NOT EXISTS (SELECT 1 FROM massage_enrollments n WHERE n.cycle_id=e.cycle_id AND n.user_id=e.user_id)`, value.ID, eligibleUserIDs); err != nil {
			return err
		}
		deliveryByUser := make(map[string]domain.MassageDelivery, len(deliveries))
		for _, delivery := range deliveries {
			deliveryByUser[delivery.UserID] = delivery
		}
		for _, userID := range eligibleUserIDs {
			inserted, insertErr := tx.Exec(ctx, `INSERT INTO massage_eligible_users(cycle_id,user_id,created_at) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, value.ID, userID, value.UpdatedAt)
			if insertErr != nil {
				return insertErr
			}
			if inserted.RowsAffected() > 0 {
				if delivery, ok := deliveryByUser[userID]; ok {
					if err = insertMassageDelivery(ctx, tx, delivery); err != nil {
						return err
					}
				}
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE massage_deliveries SET available_at=$2,updated_at=$3 WHERE cycle_id=$1 AND kind IN ('signup','signup_resend') AND status='pending'`, value.ID, value.SignupNoticeAt, value.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) DeleteMassageCycle(ctx context.Context, id string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM massage_cycles WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) PublishMassageCycle(ctx context.Context, cycleID string, userIDs []string, deliveries []domain.MassageDelivery, now time.Time) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE massage_cycles SET status='scheduled',updated_at=$2 WHERE id=$1 AND status='draft'`, cycleID, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	for _, userID := range userIDs {
		if _, err = tx.Exec(ctx, `INSERT INTO massage_eligible_users(cycle_id,user_id,created_at) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, cycleID, userID, now); err != nil {
			return err
		}
	}
	for _, delivery := range deliveries {
		if err = insertMassageDelivery(ctx, tx, delivery); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (p *Postgres) ListMassageEligibleUserIDs(ctx context.Context, cycleID string) ([]string, error) {
	rows, err := p.pool.Query(ctx, `SELECT user_id FROM massage_eligible_users WHERE cycle_id=$1 ORDER BY created_at,user_id`, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func insertMassageDelivery(ctx context.Context, tx pgx.Tx, value domain.MassageDelivery) error {
	_, err := tx.Exec(ctx, `INSERT INTO massage_deliveries(id,cycle_id,session_id,call_id,user_id,kind,status,attempts,available_at,idempotency_key,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11) ON CONFLICT(idempotency_key) DO NOTHING`, value.ID, value.CycleID, nullUUID(value.SessionID), nullUUID(value.CallID), value.UserID, value.Kind, value.Status, value.Attempts, value.AvailableAt, value.IdempotencyKey, value.CreatedAt)
	return err
}

func (p *Postgres) MassageResponse(ctx context.Context, cycleID, userID, action string, now time.Time) (domain.MassageEnrollment, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.MassageEnrollment{}, err
	}
	defer tx.Rollback(ctx)
	var status string
	var deadline time.Time
	var next int
	err = tx.QueryRow(ctx, `SELECT status,signup_deadline,next_queue FROM massage_cycles WHERE id=$1 FOR UPDATE`, cycleID).Scan(&status, &deadline, &next)
	if err == pgx.ErrNoRows {
		return domain.MassageEnrollment{}, ErrNotFound
	}
	if err != nil {
		return domain.MassageEnrollment{}, err
	}
	if status != "signup_open" || now.After(deadline) {
		return domain.MassageEnrollment{}, ErrConflict
	}
	var eligible bool
	_ = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM massage_eligible_users WHERE cycle_id=$1 AND user_id=$2)`, cycleID, userID).Scan(&eligible)
	if !eligible {
		return domain.MassageEnrollment{}, ErrForbidden
	}
	var value domain.MassageEnrollment
	_ = tx.QueryRow(ctx, `SELECT id,cycle_id,user_id,COALESCE(queue_number,0),status,enrolled_at,withdrawn_at,created_at,updated_at FROM massage_enrollments WHERE cycle_id=$1 AND user_id=$2`, cycleID, userID).Scan(&value.ID, &value.CycleID, &value.UserID, &value.QueueNumber, &value.Status, &value.EnrolledAt, &value.WithdrawnAt, &value.CreatedAt, &value.UpdatedAt)
	if value.ID == "" {
		value.ID = ids.New("men")
		value.CycleID = cycleID
		value.UserID = userID
		value.CreatedAt = now
	}
	switch action {
	case "enroll":
		if value.Status == "enrolled" {
			_ = tx.Commit(ctx)
			return value, nil
		}
		next++
		value.QueueNumber = next
		value.Status = "enrolled"
		value.EnrolledAt = &now
		value.WithdrawnAt = nil
		_, err = tx.Exec(ctx, `UPDATE massage_cycles SET next_queue=$2,updated_at=$3 WHERE id=$1`, cycleID, next, now)
	case "decline":
		value.QueueNumber = 0
		value.Status = "declined"
		value.EnrolledAt = nil
		value.WithdrawnAt = nil
	case "withdraw":
		var firstStart time.Time
		_ = tx.QueryRow(ctx, `SELECT starts_at FROM massage_sessions WHERE cycle_id=$1 AND sequence=1`, cycleID).Scan(&firstStart)
		if value.Status != "enrolled" || !now.Before(firstStart) {
			return value, ErrConflict
		}
		value.Status = "withdrawn"
		value.WithdrawnAt = &now
	default:
		return value, fmt.Errorf("invalid massage response")
	}
	if err != nil {
		return value, err
	}
	value.UpdatedAt = now
	_, err = tx.Exec(ctx, `INSERT INTO massage_enrollments(id,cycle_id,user_id,queue_number,status,enrolled_at,withdrawn_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(cycle_id,user_id) DO UPDATE SET queue_number=EXCLUDED.queue_number,status=EXCLUDED.status,enrolled_at=EXCLUDED.enrolled_at,withdrawn_at=EXCLUDED.withdrawn_at,updated_at=EXCLUDED.updated_at`, value.ID, cycleID, userID, nullInt(value.QueueNumber), value.Status, value.EnrolledAt, value.WithdrawnAt, value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}

func nullInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}

func (p *Postgres) ListMassageMe(ctx context.Context, userID string) ([]domain.MassageMe, error) {
	rows, err := p.pool.Query(ctx, `SELECT c.id FROM massage_cycles c JOIN massage_eligible_users e ON e.cycle_id=c.id AND e.user_id=$1 WHERE c.status IN ('scheduled','signup_open','in_progress') ORDER BY c.service_month,c.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	idsList := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		idsList = append(idsList, id)
	}
	values := []domain.MassageMe{}
	for _, cycleID := range idsList {
		cycle, err := p.GetMassageCycle(ctx, cycleID)
		if err != nil {
			return nil, err
		}
		item := domain.MassageMe{Cycle: cycle}
		var enrollment domain.MassageEnrollment
		err = p.pool.QueryRow(ctx, `SELECT e.id,e.cycle_id,e.user_id,u.name,COALESCE(e.queue_number,0),e.status,e.enrolled_at,e.withdrawn_at,e.created_at,e.updated_at FROM massage_enrollments e JOIN users u ON u.id=e.user_id WHERE e.cycle_id=$1 AND e.user_id=$2`, cycleID, userID).Scan(&enrollment.ID, &enrollment.CycleID, &enrollment.UserID, &enrollment.UserName, &enrollment.QueueNumber, &enrollment.Status, &enrollment.EnrolledAt, &enrollment.WithdrawnAt, &enrollment.CreatedAt, &enrollment.UpdatedAt)
		if err == nil {
			item.Enrollment = &enrollment
		}
		var call domain.MassageCall
		err = p.pool.QueryRow(ctx, massageCallSelect+`WHERE e.cycle_id=$1 AND e.user_id=$2 ORDER BY mc.created_at DESC LIMIT 1`, cycleID, userID).Scan(massageCallDest(&call)...)
		if err == nil {
			item.CurrentCall = &call
		}
		_ = p.pool.QueryRow(ctx, `SELECT COALESCE(min(e.queue_number),0) FROM massage_calls mc JOIN massage_enrollments e ON e.id=mc.enrollment_id WHERE e.cycle_id=$1 AND mc.status IN ('awaiting_response','accepted')`, cycleID).Scan(&item.CurrentQueueNumber)
		values = append(values, item)
	}
	return values, nil
}

func (p *Postgres) CreateMassageResendDeliveries(ctx context.Context, cycleID string, deliveries []domain.MassageDelivery) (int, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	count := 0
	for _, d := range deliveries {
		tag, execErr := tx.Exec(ctx, `INSERT INTO massage_deliveries(id,cycle_id,user_id,kind,status,available_at,idempotency_key,created_at,updated_at) SELECT $1,$2,$3,'signup_resend','pending',$4,$5,$4,$4 WHERE EXISTS(SELECT 1 FROM massage_eligible_users WHERE cycle_id=$2 AND user_id=$3) AND NOT EXISTS(SELECT 1 FROM massage_enrollments WHERE cycle_id=$2 AND user_id=$3 AND status='enrolled') ON CONFLICT(idempotency_key) DO NOTHING`, d.ID, cycleID, d.UserID, d.AvailableAt, d.IdempotencyKey)
		if execErr != nil {
			return count, execErr
		}
		count += int(tag.RowsAffected())
	}
	return count, tx.Commit(ctx)
}

func (p *Postgres) SetMassageSessionStatus(ctx context.Context, id, status, reason string, now time.Time) (domain.MassageSession, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.MassageSession{}, err
	}
	defer tx.Rollback(ctx)
	var current string
	var cycleID string
	var sequence int
	err = tx.QueryRow(ctx, `SELECT status,cycle_id,sequence FROM massage_sessions WHERE id=$1 FOR UPDATE`, id).Scan(&current, &cycleID, &sequence)
	if err == pgx.ErrNoRows {
		return domain.MassageSession{}, ErrNotFound
	}
	if err != nil {
		return domain.MassageSession{}, err
	}
	allowed := false
	switch status {
	case "running":
		allowed = current == "scheduled" || current == "paused"
	case "paused":
		allowed = current == "running"
	case "closed":
		allowed = current == "running" || current == "paused"
	}
	if !allowed {
		return domain.MassageSession{}, ErrConflict
	}
	if status == "running" {
		_, err = tx.Exec(ctx, `UPDATE massage_sessions SET status='running',started_at=COALESCE(started_at,$2),updated_at=$2 WHERE id=$1`, id, now)
		_, _ = tx.Exec(ctx, `UPDATE massage_cycles SET status='in_progress',updated_at=$2 WHERE id=$1`, cycleID, now)
	} else if status == "paused" {
		_, err = tx.Exec(ctx, `UPDATE massage_sessions SET status='paused',updated_at=$2 WHERE id=$1`, id, now)
	} else {
		var active int
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM massage_calls WHERE session_id=$1 AND status IN ('pending_delivery','awaiting_response','accepted')`, id).Scan(&active)
		if active > 0 {
			return domain.MassageSession{}, ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE massage_sessions SET status='closed',closed_at=$2,close_reason=$3,updated_at=$2 WHERE id=$1`, id, now, reason)
		if sequence == 2 {
			_, _ = tx.Exec(ctx, `UPDATE massage_enrollments SET status='unserved',updated_at=$2 WHERE cycle_id=$1 AND status='enrolled'`, cycleID, now)
			_, _ = tx.Exec(ctx, `UPDATE massage_cycles SET status='completed',updated_at=$2 WHERE id=$1`, cycleID, now)
		}
	}
	if err != nil {
		return domain.MassageSession{}, err
	}
	var value domain.MassageSession
	err = tx.QueryRow(ctx, `SELECT id,cycle_id,sequence,starts_at,quota,concurrent_slots,status,completed_count,started_at,closed_at,close_reason,created_at,updated_at FROM massage_sessions WHERE id=$1`, id).Scan(&value.ID, &value.CycleID, &value.Sequence, &value.StartsAt, &value.Quota, &value.ConcurrentSlots, &value.Status, &value.CompletedCount, &value.StartedAt, &value.ClosedAt, &value.CloseReason, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}

func (p *Postgres) FillMassageSession(ctx context.Context, sessionID string, now time.Time) ([]domain.MassageCall, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var session domain.MassageSession
	err = tx.QueryRow(ctx, `SELECT id,cycle_id,sequence,starts_at,quota,concurrent_slots,status,completed_count,started_at,closed_at,close_reason,created_at,updated_at FROM massage_sessions WHERE id=$1 FOR UPDATE`, sessionID).Scan(&session.ID, &session.CycleID, &session.Sequence, &session.StartsAt, &session.Quota, &session.ConcurrentSlots, &session.Status, &session.CompletedCount, &session.StartedAt, &session.ClosedAt, &session.CloseReason, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if session.Status != "running" {
		return []domain.MassageCall{}, tx.Commit(ctx)
	}
	var active int
	_ = tx.QueryRow(ctx, `SELECT count(*) FROM massage_calls WHERE session_id=$1 AND status IN ('pending_delivery','awaiting_response','accepted')`, sessionID).Scan(&active)
	slots := session.ConcurrentSlots - active
	remaining := session.Quota - session.CompletedCount - active
	if remaining < slots {
		slots = remaining
	}
	if slots <= 0 {
		return []domain.MassageCall{}, tx.Commit(ctx)
	}
	rows, err := tx.Query(ctx, `SELECT e.id,e.user_id,e.queue_number FROM massage_enrollments e WHERE e.cycle_id=$1 AND e.status='enrolled' AND NOT EXISTS(SELECT 1 FROM massage_calls mc WHERE mc.session_id=$2 AND mc.enrollment_id=e.id) ORDER BY e.queue_number FOR UPDATE OF e SKIP LOCKED LIMIT $3`, session.CycleID, sessionID, slots)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		id, user string
		queue    int
	}
	candidates := []candidate{}
	for rows.Next() {
		var c candidate
		if err = rows.Scan(&c.id, &c.user, &c.queue); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, c)
	}
	rows.Close()
	values := []domain.MassageCall{}
	for _, c := range candidates {
		call := domain.MassageCall{ID: ids.New("mcl"), SessionID: sessionID, EnrollmentID: c.id, UserID: c.user, QueueNumber: c.queue, Status: "pending_delivery", CreatedAt: now, UpdatedAt: now}
		_, err = tx.Exec(ctx, `INSERT INTO massage_calls(id,session_id,enrollment_id,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$5)`, call.ID, sessionID, c.id, call.Status, now)
		if err != nil {
			return nil, err
		}
		delivery := domain.MassageDelivery{ID: ids.New("mdl"), CycleID: session.CycleID, SessionID: sessionID, CallID: call.ID, UserID: c.user, Kind: "call", Status: "pending", AvailableAt: now, IdempotencyKey: shortIdempotency("massage-call-" + call.ID), CreatedAt: now}
		if err = insertMassageDelivery(ctx, tx, delivery); err != nil {
			return nil, err
		}
		values = append(values, call)
	}
	return values, tx.Commit(ctx)
}

func shortIdempotency(value string) string {
	if len(value) > 50 {
		return value[:50]
	}
	return value
}

const massageDeliverySelect = `SELECT id,cycle_id,COALESCE(session_id::text,''),COALESCE(call_id::text,''),user_id,kind,status,attempts,available_at,locked_until,message_id,last_error,idempotency_key,created_at,updated_at FROM massage_deliveries `

func (p *Postgres) ClaimMassageDeliveries(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.MassageDelivery, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	_, _ = tx.Exec(ctx, `UPDATE massage_cycles SET status='signup_open',updated_at=$1 WHERE status='scheduled' AND signup_notice_at<=$1 AND signup_deadline>$1`, now)
	rows, err := tx.Query(ctx, `WITH due AS(SELECT id FROM massage_deliveries WHERE status IN ('pending','retry','sending') AND available_at<=$1 AND (locked_until IS NULL OR locked_until<$1) ORDER BY available_at FOR UPDATE SKIP LOCKED LIMIT $2) UPDATE massage_deliveries d SET status='sending',attempts=d.attempts+1,locked_until=$1+$3::interval,updated_at=$1 FROM due WHERE d.id=due.id RETURNING d.id,d.cycle_id,COALESCE(d.session_id::text,''),COALESCE(d.call_id::text,''),d.user_id,d.kind,d.status,d.attempts,d.available_at,d.locked_until,d.message_id,d.last_error,d.idempotency_key,d.created_at,d.updated_at`, now, limit, lease.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.MassageDelivery{}
	for rows.Next() {
		var v domain.MassageDelivery
		if err = rows.Scan(&v.ID, &v.CycleID, &v.SessionID, &v.CallID, &v.UserID, &v.Kind, &v.Status, &v.Attempts, &v.AvailableAt, &v.LockedUntil, &v.MessageID, &v.LastError, &v.IdempotencyKey, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return values, tx.Commit(ctx)
}
func (p *Postgres) UpdateMassageDelivery(ctx context.Context, v domain.MassageDelivery) error {
	_, err := p.pool.Exec(ctx, `UPDATE massage_deliveries SET status=$2,attempts=$3,available_at=$4,locked_until=$5,message_id=$6,last_error=$7,updated_at=$8 WHERE id=$1`, v.ID, v.Status, v.Attempts, v.AvailableAt, v.LockedUntil, v.MessageID, v.LastError, v.UpdatedAt)
	return err
}
func (p *Postgres) MarkMassageCallSent(ctx context.Context, id, messageID string, called, due time.Time) error {
	_, err := p.pool.Exec(ctx, `UPDATE massage_calls SET status='awaiting_response',message_id=$2,called_at=$3,response_due_at=$4,updated_at=$3 WHERE id=$1 AND status='pending_delivery'`, id, messageID, called, due)
	return err
}
func (p *Postgres) MarkMassageCallDeliveryFailed(ctx context.Context, id, message string, now time.Time) error {
	_, err := p.pool.Exec(ctx, `UPDATE massage_calls SET status='delivery_failed',last_error=$2,updated_at=$3 WHERE id=$1 AND status='pending_delivery'`, id, message, now)
	return err
}
func (p *Postgres) ExpireMassageCalls(ctx context.Context, now time.Time) ([]domain.MassageCall, error) {
	rows, err := p.pool.Query(ctx, `UPDATE massage_calls SET status='timed_out',updated_at=$1 WHERE status='awaiting_response' AND response_due_at<$1 RETURNING id`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	idsList := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		idsList = append(idsList, id)
	}
	out := []domain.MassageCall{}
	for _, id := range idsList {
		v, getErr := p.GetMassageCall(ctx, id)
		if getErr == nil {
			out = append(out, v)
		}
	}
	return out, rows.Err()
}

const massageCallSelect = `SELECT mc.id,mc.session_id,mc.enrollment_id,e.user_id,u.name,e.queue_number,mc.status,mc.called_at,mc.response_due_at,mc.responded_at,mc.completed_at,mc.message_id,mc.last_error,mc.created_at,mc.updated_at FROM massage_calls mc JOIN massage_enrollments e ON e.id=mc.enrollment_id JOIN users u ON u.id=e.user_id `

func massageCallDest(v *domain.MassageCall) []any {
	return []any{&v.ID, &v.SessionID, &v.EnrollmentID, &v.UserID, &v.UserName, &v.QueueNumber, &v.Status, &v.CalledAt, &v.ResponseDueAt, &v.RespondedAt, &v.CompletedAt, &v.MessageID, &v.LastError, &v.CreatedAt, &v.UpdatedAt}
}
func (p *Postgres) GetMassageCall(ctx context.Context, id string) (domain.MassageCall, error) {
	var v domain.MassageCall
	err := p.pool.QueryRow(ctx, massageCallSelect+`WHERE mc.id=$1`, id).Scan(massageCallDest(&v)...)
	if err == pgx.ErrNoRows {
		err = ErrNotFound
	}
	return v, err
}
func (p *Postgres) RespondMassageCall(ctx context.Context, id, userID, action string, now time.Time) (domain.MassageCall, error) {
	status := "accepted"
	if action == "reject" {
		status = "rejected"
	} else if action != "accept" {
		return domain.MassageCall{}, fmt.Errorf("invalid call response")
	}
	tag, err := p.pool.Exec(ctx, `UPDATE massage_calls mc SET status=$3,responded_at=$4,updated_at=$4 FROM massage_enrollments e WHERE mc.id=$1 AND mc.enrollment_id=e.id AND e.user_id=$2 AND mc.status='awaiting_response' AND mc.response_due_at>=$4`, id, userID, status, now)
	if err != nil {
		return domain.MassageCall{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.MassageCall{}, ErrConflict
	}
	return p.GetMassageCall(ctx, id)
}
func (p *Postgres) CompleteMassageCall(ctx context.Context, id, action string, now time.Time) (domain.MassageCall, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.MassageCall{}, err
	}
	defer tx.Rollback(ctx)
	target := "completed"
	if action == "no_show" {
		target = "no_show"
	} else if action != "complete" {
		return domain.MassageCall{}, fmt.Errorf("invalid admin call action")
	}
	var sessionID, enrollmentID string
	err = tx.QueryRow(ctx, `UPDATE massage_calls SET status=$2,completed_at=CASE WHEN $2='completed' THEN $3 ELSE completed_at END,updated_at=$3 WHERE id=$1 AND status='accepted' RETURNING session_id,enrollment_id`, id, target, now).Scan(&sessionID, &enrollmentID)
	if err == pgx.ErrNoRows {
		return domain.MassageCall{}, ErrConflict
	}
	if err != nil {
		return domain.MassageCall{}, err
	}
	if target == "completed" {
		var cycleID string
		var sequence, completed, quota int
		err = tx.QueryRow(ctx, `UPDATE massage_sessions SET completed_count=completed_count+1,updated_at=$2 WHERE id=$1 RETURNING cycle_id,sequence,completed_count,quota`, sessionID, now).Scan(&cycleID, &sequence, &completed, &quota)
		if err != nil {
			return domain.MassageCall{}, err
		}
		_, _ = tx.Exec(ctx, `UPDATE massage_enrollments SET status='completed',updated_at=$2 WHERE id=$1`, enrollmentID, now)
		if completed >= quota {
			_, _ = tx.Exec(ctx, `UPDATE massage_sessions SET status='completed',closed_at=$2,updated_at=$2 WHERE id=$1`, sessionID, now)
			if sequence == 2 {
				_, _ = tx.Exec(ctx, `UPDATE massage_enrollments SET status='unserved',updated_at=$2 WHERE cycle_id=$1 AND status='enrolled'`, cycleID, now)
				_, _ = tx.Exec(ctx, `UPDATE massage_cycles SET status='completed',updated_at=$2 WHERE id=$1`, cycleID, now)
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.MassageCall{}, err
	}
	return p.GetMassageCall(ctx, id)
}

func (p *Postgres) MassageStatistics(ctx context.Context, cycleID string) (domain.MassageStatistic, error) {
	result := domain.MassageStatistic{CycleID: cycleID}
	err := p.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM massage_eligible_users WHERE cycle_id=$1),(SELECT count(*) FROM massage_enrollments WHERE cycle_id=$1 AND queue_number IS NOT NULL),(SELECT count(*) FROM massage_calls mc JOIN massage_sessions s ON s.id=mc.session_id WHERE s.cycle_id=$1 AND mc.status='completed'),(SELECT count(*) FROM massage_calls mc JOIN massage_sessions s ON s.id=mc.session_id WHERE s.cycle_id=$1 AND mc.status='rejected'),(SELECT count(*) FROM massage_calls mc JOIN massage_sessions s ON s.id=mc.session_id WHERE s.cycle_id=$1 AND mc.status='timed_out'),(SELECT count(*) FROM massage_calls mc JOIN massage_sessions s ON s.id=mc.session_id WHERE s.cycle_id=$1 AND mc.status='no_show'),(SELECT count(*) FROM massage_calls mc JOIN massage_sessions s ON s.id=mc.session_id WHERE s.cycle_id=$1 AND mc.status='delivery_failed'),(SELECT count(*) FROM massage_enrollments WHERE cycle_id=$1 AND status='unserved')`, cycleID).Scan(&result.Eligible, &result.Enrolled, &result.Completed, &result.Rejected, &result.TimedOut, &result.NoShow, &result.DeliveryFail, &result.Unserved)
	if err != nil {
		return result, err
	}
	rows, err := p.pool.Query(ctx, `SELECT e.id,e.cycle_id,e.user_id,u.name,COALESCE(e.queue_number,0),e.status,e.enrolled_at,e.withdrawn_at,e.created_at,e.updated_at FROM massage_enrollments e JOIN users u ON u.id=e.user_id WHERE e.cycle_id=$1 ORDER BY e.queue_number NULLS LAST,u.name`, cycleID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var v domain.MassageEnrollment
		if err = rows.Scan(&v.ID, &v.CycleID, &v.UserID, &v.UserName, &v.QueueNumber, &v.Status, &v.EnrolledAt, &v.WithdrawnAt, &v.CreatedAt, &v.UpdatedAt); err != nil {
			rows.Close()
			return result, err
		}
		result.Enrollments = append(result.Enrollments, v)
	}
	rows.Close()
	rows, err = p.pool.Query(ctx, massageCallSelect+`JOIN massage_sessions s ON s.id=mc.session_id WHERE s.cycle_id=$1 ORDER BY mc.created_at`, cycleID)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var v domain.MassageCall
		if err = rows.Scan(massageCallDest(&v)...); err != nil {
			rows.Close()
			return result, err
		}
		result.Calls = append(result.Calls, v)
	}
	rows.Close()
	return result, nil
}
func (p *Postgres) CleanupMassageHistory(ctx context.Context, before time.Time) error {
	_, err := p.pool.Exec(ctx, `WITH old AS(SELECT id,service_month FROM massage_cycles WHERE status IN ('completed','cancelled') AND updated_at<$1), stats AS(INSERT INTO massage_monthly_statistics(service_month,eligible_count,enrolled_count,completed_count,rejected_count,timed_out_count,no_show_count,delivery_failed_count,updated_at) SELECT o.service_month,(SELECT count(*) FROM massage_eligible_users WHERE cycle_id=o.id),(SELECT count(*) FROM massage_enrollments WHERE cycle_id=o.id AND queue_number IS NOT NULL),(SELECT count(*) FROM massage_calls mc JOIN massage_sessions s ON s.id=mc.session_id WHERE s.cycle_id=o.id AND mc.status='completed'),(SELECT count(*) FROM massage_calls mc JOIN massage_sessions s ON s.id=mc.session_id WHERE s.cycle_id=o.id AND mc.status='rejected'),(SELECT count(*) FROM massage_calls mc JOIN massage_sessions s ON s.id=mc.session_id WHERE s.cycle_id=o.id AND mc.status='timed_out'),(SELECT count(*) FROM massage_calls mc JOIN massage_sessions s ON s.id=mc.session_id WHERE s.cycle_id=o.id AND mc.status='no_show'),(SELECT count(*) FROM massage_calls mc JOIN massage_sessions s ON s.id=mc.session_id WHERE s.cycle_id=o.id AND mc.status='delivery_failed'),now() FROM old o ON CONFLICT(service_month) DO UPDATE SET eligible_count=EXCLUDED.eligible_count,enrolled_count=EXCLUDED.enrolled_count,completed_count=EXCLUDED.completed_count,rejected_count=EXCLUDED.rejected_count,timed_out_count=EXCLUDED.timed_out_count,no_show_count=EXCLUDED.no_show_count,delivery_failed_count=EXCLUDED.delivery_failed_count,updated_at=now()) DELETE FROM massage_cycles WHERE id IN(SELECT id FROM old)`, before)
	return err
}
