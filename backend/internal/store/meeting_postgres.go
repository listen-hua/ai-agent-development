package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"internal-ai-agent/backend/internal/domain"
)

func (p *Postgres) UpsertMeetingRooms(ctx context.Context, rooms []domain.MeetingRoom) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	roomIDs := make([]string, 0, len(rooms))
	for _, room := range rooms {
		roomIDs = append(roomIDs, room.RoomID)
		_, err = tx.Exec(ctx, `INSERT INTO meeting_rooms(id,room_id,name,capacity,room_level_id,path,enabled,schedule_enabled,disabled_from,disabled_until,disable_reason,approval_switch,approval_condition,approval_duration_hours,reservation_start_seconds,reservation_end_seconds,max_duration_hours,last_error,synced_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
			ON CONFLICT(room_id) DO UPDATE SET name=excluded.name,capacity=excluded.capacity,room_level_id=excluded.room_level_id,path=excluded.path,enabled=excluded.enabled,schedule_enabled=excluded.schedule_enabled,disabled_from=excluded.disabled_from,disabled_until=excluded.disabled_until,disable_reason=excluded.disable_reason,approval_switch=excluded.approval_switch,approval_condition=excluded.approval_condition,approval_duration_hours=excluded.approval_duration_hours,reservation_start_seconds=excluded.reservation_start_seconds,reservation_end_seconds=excluded.reservation_end_seconds,max_duration_hours=excluded.max_duration_hours,last_error=excluded.last_error,synced_at=excluded.synced_at`, room.ID, room.RoomID, room.Name, room.Capacity, room.RoomLevelID, mustJSON(room.Path), room.Enabled, room.ScheduleEnabled, room.DisabledFrom, room.DisabledUntil, room.DisableReason, room.ApprovalSwitch, room.ApprovalCondition, room.ApprovalDurationHours, room.ReservationStartSeconds, room.ReservationEndSeconds, room.MaxDurationHours, room.LastError, room.SyncedAt)
		if err != nil {
			return err
		}
	}
	if len(roomIDs) == 0 {
		_, err = tx.Exec(ctx, `UPDATE meeting_rooms SET enabled=false,schedule_enabled=false,last_error='会议室已从飞书同步范围移除'`)
	} else {
		_, err = tx.Exec(ctx, `UPDATE meeting_rooms SET enabled=false,schedule_enabled=false,last_error='会议室已从飞书同步范围移除' WHERE NOT (room_id=ANY($1))`, roomIDs)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const meetingRoomSelect = `SELECT id,room_id,name,capacity,room_level_id,path,enabled,schedule_enabled,disabled_from,disabled_until,disable_reason,approval_switch,approval_condition,approval_duration_hours,reservation_start_seconds,reservation_end_seconds,max_duration_hours,last_error,synced_at FROM meeting_rooms`

func scanMeetingRoom(row rowScanner) (domain.MeetingRoom, error) {
	var value domain.MeetingRoom
	var path []byte
	if err := row.Scan(&value.ID, &value.RoomID, &value.Name, &value.Capacity, &value.RoomLevelID, &path, &value.Enabled, &value.ScheduleEnabled, &value.DisabledFrom, &value.DisabledUntil, &value.DisableReason, &value.ApprovalSwitch, &value.ApprovalCondition, &value.ApprovalDurationHours, &value.ReservationStartSeconds, &value.ReservationEndSeconds, &value.MaxDurationHours, &value.LastError, &value.SyncedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return value, ErrNotFound
		}
		return value, err
	}
	_ = json.Unmarshal(path, &value.Path)
	return value, nil
}

func (p *Postgres) ListMeetingRooms(ctx context.Context) ([]domain.MeetingRoom, error) {
	rows, err := p.pool.Query(ctx, meetingRoomSelect+` ORDER BY capacity,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.MeetingRoom{}
	for rows.Next() {
		value, scanErr := scanMeetingRoom(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (p *Postgres) GetMeetingRoom(ctx context.Context, roomID string) (domain.MeetingRoom, error) {
	return scanMeetingRoom(p.pool.QueryRow(ctx, meetingRoomSelect+` WHERE room_id=$1`, roomID))
}

func (p *Postgres) GetMeetingSettings(ctx context.Context) (domain.MeetingSettings, error) {
	var value domain.MeetingSettings
	err := p.pool.QueryRow(ctx, `SELECT calendar_id,timezone,workday_start,workday_end,slot_minutes,sync_interval_minutes,last_synced_at,last_sync_error,updated_at FROM meeting_settings WHERE singleton=true`).Scan(&value.CalendarID, &value.Timezone, &value.WorkdayStart, &value.WorkdayEnd, &value.SlotMinutes, &value.SyncIntervalMinute, &value.LastSyncedAt, &value.LastSyncError, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	return value, err
}

func (p *Postgres) SaveMeetingSettings(ctx context.Context, value domain.MeetingSettings) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO meeting_settings(singleton,calendar_id,timezone,workday_start,workday_end,slot_minutes,sync_interval_minutes,last_synced_at,last_sync_error,updated_at) VALUES(true,$1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(singleton) DO UPDATE SET calendar_id=excluded.calendar_id,timezone=excluded.timezone,workday_start=excluded.workday_start,workday_end=excluded.workday_end,slot_minutes=excluded.slot_minutes,sync_interval_minutes=excluded.sync_interval_minutes,last_synced_at=excluded.last_synced_at,last_sync_error=excluded.last_sync_error,updated_at=excluded.updated_at`, value.CalendarID, value.Timezone, value.WorkdayStart, value.WorkdayEnd, value.SlotMinutes, value.SyncIntervalMinute, value.LastSyncedAt, value.LastSyncError, value.UpdatedAt)
	return err
}

func (p *Postgres) CreateMeetingBookingAction(ctx context.Context, value domain.MeetingBookingAction) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO meeting_booking_actions(id,user_id,intent,title,attendees,capacity,requested_room_name,options,selected_option_id,booking_id,result_booking_id,status,source_channel,source_conversation_id,expires_at,created_at,confirmed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`, value.ID, value.UserID, value.Intent, value.Title, mustJSON(value.Attendees), value.Capacity, value.RequestedRoomName, mustJSON(value.Options), nullUUID(value.SelectedOptionID), nullUUID(value.BookingID), nullUUID(value.ResultBookingID), value.Status, value.SourceChannel, nullUUID(value.SourceConversationID), value.ExpiresAt, value.CreatedAt, value.ConfirmedAt)
	return err
}

const meetingActionSelect = `SELECT id,user_id,intent,title,attendees,capacity,requested_room_name,options,COALESCE(selected_option_id::text,''),COALESCE(booking_id::text,''),COALESCE(result_booking_id::text,''),status,source_channel,COALESCE(source_conversation_id::text,''),expires_at,created_at,confirmed_at FROM meeting_booking_actions`

func scanMeetingAction(row rowScanner) (domain.MeetingBookingAction, error) {
	var value domain.MeetingBookingAction
	var attendees, options []byte
	err := row.Scan(&value.ID, &value.UserID, &value.Intent, &value.Title, &attendees, &value.Capacity, &value.RequestedRoomName, &options, &value.SelectedOptionID, &value.BookingID, &value.ResultBookingID, &value.Status, &value.SourceChannel, &value.SourceConversationID, &value.ExpiresAt, &value.CreatedAt, &value.ConfirmedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	if err != nil {
		return value, err
	}
	_ = json.Unmarshal(attendees, &value.Attendees)
	_ = json.Unmarshal(options, &value.Options)
	return value, nil
}

func (p *Postgres) GetMeetingBookingAction(ctx context.Context, id string) (domain.MeetingBookingAction, error) {
	return scanMeetingAction(p.pool.QueryRow(ctx, meetingActionSelect+` WHERE id=$1`, id))
}

func (p *Postgres) ClaimMeetingBookingAction(ctx context.Context, id, userID, optionID string, now time.Time) (domain.MeetingBookingAction, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.MeetingBookingAction{}, err
	}
	defer tx.Rollback(ctx)
	value, err := scanMeetingAction(tx.QueryRow(ctx, meetingActionSelect+` WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return value, err
	}
	if value.UserID != userID {
		return value, ErrForbidden
	}
	if value.Status != domain.MeetingActionPending || now.After(value.ExpiresAt) {
		if value.Status == domain.MeetingActionPending {
			_, _ = tx.Exec(ctx, `UPDATE meeting_booking_actions SET status='expired' WHERE id=$1`, id)
			_ = tx.Commit(ctx)
		}
		return value, ErrConflict
	}
	value.Status = domain.MeetingActionProcessing
	value.SelectedOptionID = optionID
	_, err = tx.Exec(ctx, `UPDATE meeting_booking_actions SET status='processing',selected_option_id=$2 WHERE id=$1`, id, nullUUID(optionID))
	if err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}

func (p *Postgres) UpdateMeetingBookingAction(ctx context.Context, value domain.MeetingBookingAction) error {
	tag, err := p.pool.Exec(ctx, `UPDATE meeting_booking_actions SET selected_option_id=$2,result_booking_id=$3,status=$4,confirmed_at=$5 WHERE id=$1`, value.ID, nullUUID(value.SelectedOptionID), nullUUID(value.ResultBookingID), value.Status, value.ConfirmedAt)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) CreateMeetingBooking(ctx context.Context, value domain.MeetingBooking) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO meeting_bookings(id,user_id,calendar_id,event_id,room_id,room_name,title,start_at,end_at,attendees,status,replaces_booking_id,replaced_by_booking_id,last_error,source_channel,source_conversation_id,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, value.ID, value.UserID, value.CalendarID, value.EventID, value.RoomID, value.RoomName, value.Title, value.StartAt, value.EndAt, mustJSON(value.Attendees), value.Status, nullUUID(value.ReplacesBookingID), nullUUID(value.ReplacedByBookingID), value.LastError, value.SourceChannel, nullUUID(value.SourceConversationID), value.CreatedAt, value.UpdatedAt)
	return err
}

const meetingBookingSelect = `SELECT id,user_id,calendar_id,event_id,room_id,room_name,title,start_at,end_at,attendees,status,COALESCE(replaces_booking_id::text,''),COALESCE(replaced_by_booking_id::text,''),last_error,source_channel,COALESCE(source_conversation_id::text,''),created_at,updated_at FROM meeting_bookings`

func scanMeetingBooking(row rowScanner) (domain.MeetingBooking, error) {
	var value domain.MeetingBooking
	var attendees []byte
	err := row.Scan(&value.ID, &value.UserID, &value.CalendarID, &value.EventID, &value.RoomID, &value.RoomName, &value.Title, &value.StartAt, &value.EndAt, &attendees, &value.Status, &value.ReplacesBookingID, &value.ReplacedByBookingID, &value.LastError, &value.SourceChannel, &value.SourceConversationID, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	if err != nil {
		return value, err
	}
	_ = json.Unmarshal(attendees, &value.Attendees)
	return value, nil
}

func (p *Postgres) GetMeetingBooking(ctx context.Context, id string) (domain.MeetingBooking, error) {
	return scanMeetingBooking(p.pool.QueryRow(ctx, meetingBookingSelect+` WHERE id=$1`, id))
}

func (p *Postgres) ListMeetingBookings(ctx context.Context, userID string) ([]domain.MeetingBooking, error) {
	query := meetingBookingSelect
	args := []any{}
	if userID != "" {
		query += ` WHERE user_id=$1`
		args = append(args, userID)
	}
	query += ` ORDER BY start_at DESC`
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.MeetingBooking{}
	for rows.Next() {
		value, scanErr := scanMeetingBooking(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (p *Postgres) UpdateMeetingBooking(ctx context.Context, value domain.MeetingBooking) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE meeting_bookings SET status=$2,replaces_booking_id=$3,replaced_by_booking_id=$4,last_error=$5,updated_at=$6 WHERE id=$1`, value.ID, value.Status, nullUUID(value.ReplacesBookingID), nullUUID(value.ReplacedByBookingID), value.LastError, value.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if value.Status != "active" {
		_, err = tx.Exec(ctx, `UPDATE meeting_booking_deliveries SET status='cancelled',locked_until=NULL,updated_at=$2 WHERE booking_id=$1 AND status<>'sent'`, value.ID, value.UpdatedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) CreateMeetingBookingDelivery(ctx context.Context, value domain.MeetingBookingDelivery) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO meeting_booking_deliveries(id,booking_id,scheduled_for,status,attempts,next_attempt_at,locked_until,message_id,error,idempotency_key,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT(booking_id,scheduled_for) DO NOTHING`, value.ID, value.BookingID, value.ScheduledFor, value.Status, value.Attempts, value.NextAttemptAt, value.LockedUntil, value.MessageID, value.Error, value.IdempotencyKey, value.CreatedAt, value.UpdatedAt)
	return err
}

func (p *Postgres) ClaimMeetingBookingDeliveries(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.MeetingBookingDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := p.pool.Query(ctx, `WITH picked AS (
		SELECT d.id FROM meeting_booking_deliveries d JOIN meeting_bookings b ON b.id=d.booking_id WHERE b.status='active' AND d.status IN ('pending','retry','sending') AND d.next_attempt_at<=$1 AND (d.locked_until IS NULL OR d.locked_until<$1) ORDER BY d.next_attempt_at FOR UPDATE SKIP LOCKED LIMIT $2
	) UPDATE meeting_booking_deliveries d SET status='sending',attempts=d.attempts+1,locked_until=$1+$3::interval,updated_at=$1 FROM picked WHERE d.id=picked.id
	RETURNING d.id,d.booking_id,d.scheduled_for,d.status,d.attempts,d.next_attempt_at,d.locked_until,d.message_id,d.error,d.idempotency_key,d.created_at,d.updated_at`, now, limit, fmt.Sprintf("%f seconds", lease.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.MeetingBookingDelivery{}
	for rows.Next() {
		var value domain.MeetingBookingDelivery
		if err = rows.Scan(&value.ID, &value.BookingID, &value.ScheduledFor, &value.Status, &value.Attempts, &value.NextAttemptAt, &value.LockedUntil, &value.MessageID, &value.Error, &value.IdempotencyKey, &value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (p *Postgres) UpdateMeetingBookingDelivery(ctx context.Context, value domain.MeetingBookingDelivery) error {
	tag, err := p.pool.Exec(ctx, `UPDATE meeting_booking_deliveries SET status=$2,attempts=$3,next_attempt_at=$4,locked_until=$5,message_id=$6,error=$7,updated_at=$8 WHERE id=$1`, value.ID, value.Status, value.Attempts, value.NextAttemptAt, value.LockedUntil, value.MessageID, value.Error, value.UpdatedAt)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
