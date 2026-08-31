package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"internal-ai-agent/backend/internal/domain"
)

func (p *Postgres) GetPixianBackgroundRemovalConfig(ctx context.Context) (domain.PixianBackgroundRemovalConfig, error) {
	var v domain.PixianBackgroundRemovalConfig
	err := p.pool.QueryRow(ctx, `SELECT enabled,test_mode,encrypted_api_id,encrypted_api_secret,api_id_hint,api_secret_hint,
		timeout_seconds,concurrency,max_pixels,account_state,account_credits,account_checked_at,
		COALESCE(updated_by::text,''),updated_at FROM pixian_background_removal_config WHERE id=true`).Scan(
		&v.Enabled, &v.TestMode, &v.EncryptedAPIID, &v.EncryptedAPISecret, &v.APIIDHint, &v.APISecretHint,
		&v.TimeoutSeconds, &v.Concurrency, &v.MaxPixels, &v.AccountState, &v.AccountCredits, &v.AccountCheckedAt,
		&v.UpdatedBy, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	v.HasAPIID, v.HasAPISecret = v.EncryptedAPIID != "", v.EncryptedAPISecret != ""
	return v, err
}

func (p *Postgres) SavePixianBackgroundRemovalConfig(ctx context.Context, v domain.PixianBackgroundRemovalConfig) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO pixian_background_removal_config(id,enabled,test_mode,encrypted_api_id,
		encrypted_api_secret,api_id_hint,api_secret_hint,timeout_seconds,concurrency,max_pixels,account_state,
		account_credits,account_checked_at,updated_by,updated_at) VALUES(true,$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,'')::uuid,$14)
		ON CONFLICT(id) DO UPDATE SET enabled=EXCLUDED.enabled,test_mode=EXCLUDED.test_mode,
		encrypted_api_id=EXCLUDED.encrypted_api_id,encrypted_api_secret=EXCLUDED.encrypted_api_secret,
		api_id_hint=EXCLUDED.api_id_hint,api_secret_hint=EXCLUDED.api_secret_hint,timeout_seconds=EXCLUDED.timeout_seconds,
		concurrency=EXCLUDED.concurrency,max_pixels=EXCLUDED.max_pixels,account_state=EXCLUDED.account_state,
		account_credits=EXCLUDED.account_credits,account_checked_at=EXCLUDED.account_checked_at,
		updated_by=EXCLUDED.updated_by,updated_at=EXCLUDED.updated_at`, v.Enabled, v.TestMode, v.EncryptedAPIID,
		v.EncryptedAPISecret, v.APIIDHint, v.APISecretHint, v.TimeoutSeconds, v.Concurrency, v.MaxPixels,
		v.AccountState, v.AccountCredits, v.AccountCheckedAt, v.UpdatedBy, v.UpdatedAt)
	return err
}

func (p *Postgres) CreateBackgroundRemovalJob(ctx context.Context, job domain.BackgroundRemovalJob, items []domain.BackgroundRemovalItem, nodes []domain.ImageCanvasNode, version int64) (domain.BackgroundRemovalJob, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return job, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO background_removal_jobs(id,user_id,canvas_id,status,test_mode,attempts,
		next_attempt_at,completed_count,failed_count,credits_charged,credits_calculated,error,idempotency_key,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) ON CONFLICT(user_id,idempotency_key) DO NOTHING`,
		job.ID, job.UserID, job.CanvasID, job.Status, job.TestMode, job.Attempts, job.NextAttemptAt, job.CompletedCount,
		job.FailedCount, job.CreditsCharged, job.CreditsCalculated, job.Error, job.IdempotencyKey, job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return job, err
	}
	if tag.RowsAffected() == 0 {
		_ = tx.Rollback(ctx)
		var id string
		if err = p.pool.QueryRow(ctx, `SELECT id FROM background_removal_jobs WHERE user_id=$1 AND idempotency_key=$2`, job.UserID, job.IdempotencyKey).Scan(&id); err != nil {
			return job, err
		}
		return p.GetBackgroundRemovalJob(ctx, id)
	}
	tag, err = tx.Exec(ctx, `UPDATE image_canvases SET version=version+1,updated_at=$4 WHERE id=$1 AND user_id=$2 AND version=$3 AND deleted_at IS NULL`, job.CanvasID, job.UserID, version, job.UpdatedAt)
	if err != nil {
		return job, err
	}
	if tag.RowsAffected() == 0 {
		return job, ErrConflict
	}
	for _, n := range nodes {
		_, err = tx.Exec(ctx, `INSERT INTO image_canvas_nodes(id,canvas_id,background_removal_job_id,source_node_id,output_index,status,x,y,width,height,z_index,error,
			generation_relay_name,generation_model_name,generation_model_key,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`, n.ID, n.CanvasID, n.BackgroundRemovalJobID,
			n.SourceNodeID, n.OutputIndex, n.Status, n.X, n.Y, n.Width, n.Height, n.ZIndex, n.Error,
			n.GenerationRelayName, n.GenerationModelName, n.GenerationModelKey, n.CreatedAt, n.UpdatedAt)
		if err != nil {
			return job, err
		}
	}
	for _, item := range items {
		_, err = tx.Exec(ctx, `INSERT INTO background_removal_items(id,job_id,source_node_id,source_asset_id,project_id,placeholder_node_id,status,attempts,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, item.ID, item.JobID, item.SourceNodeID, item.SourceAssetID,
			item.ProjectID, item.PlaceholderNodeID, item.Status, item.Attempts, item.CreatedAt, item.UpdatedAt)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return job, ErrConflict
			}
			return job, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return job, err
	}
	return p.GetBackgroundRemovalJob(ctx, job.ID)
}

const backgroundRemovalJobSelect = `SELECT id,user_id,COALESCE(canvas_id::text,''),status,test_mode,attempts,next_attempt_at,
	locked_until,completed_count,failed_count,credits_charged,credits_calculated,error,idempotency_key,created_at,updated_at FROM background_removal_jobs`

func scanBackgroundRemovalJob(row rowScanner) (domain.BackgroundRemovalJob, error) {
	var v domain.BackgroundRemovalJob
	err := row.Scan(&v.ID, &v.UserID, &v.CanvasID, &v.Status, &v.TestMode, &v.Attempts, &v.NextAttemptAt, &v.LockedUntil,
		&v.CompletedCount, &v.FailedCount, &v.CreditsCharged, &v.CreditsCalculated, &v.Error, &v.IdempotencyKey, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}

func (p *Postgres) GetBackgroundRemovalJob(ctx context.Context, id string) (domain.BackgroundRemovalJob, error) {
	v, err := scanBackgroundRemovalJob(p.pool.QueryRow(ctx, backgroundRemovalJobSelect+` WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	if err != nil {
		return v, err
	}
	v.Items, err = p.listBackgroundRemovalItems(ctx, id)
	return v, err
}

func (p *Postgres) listBackgroundRemovalItems(ctx context.Context, jobID string) ([]domain.BackgroundRemovalItem, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,job_id,COALESCE(source_node_id::text,''),source_asset_id,project_id,
		COALESCE(placeholder_node_id::text,''),COALESCE(result_asset_id::text,''),status,attempts,credits_charged,
		credits_calculated,input_size,result_size,error,created_at,updated_at FROM background_removal_items WHERE job_id=$1 ORDER BY created_at,id`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.BackgroundRemovalItem{}
	for rows.Next() {
		var v domain.BackgroundRemovalItem
		if err = rows.Scan(&v.ID, &v.JobID, &v.SourceNodeID, &v.SourceAssetID, &v.ProjectID, &v.PlaceholderNodeID, &v.ResultAssetID, &v.Status, &v.Attempts, &v.CreditsCharged, &v.CreditsCalculated, &v.InputSize, &v.ResultSize, &v.Error, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (p *Postgres) ListBackgroundRemovalJobs(ctx context.Context, limit int) ([]domain.BackgroundRemovalJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := p.pool.Query(ctx, backgroundRemovalJobSelect+` ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.BackgroundRemovalJob{}
	for rows.Next() {
		v, e := scanBackgroundRemovalJob(rows)
		if e != nil {
			return nil, e
		}
		v.Items, e = p.listBackgroundRemovalItems(ctx, v.ID)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (p *Postgres) ClaimBackgroundRemovalJobs(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.BackgroundRemovalJob, error) {
	rows, err := p.pool.Query(ctx, `WITH candidates AS (SELECT id FROM background_removal_jobs WHERE status IN ('pending','retry') AND next_attempt_at<=$1 AND (locked_until IS NULL OR locked_until<$1) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $2)
		UPDATE background_removal_jobs j SET status='running',attempts=j.attempts+1,locked_until=$1+$3::interval,updated_at=$1 FROM candidates c WHERE j.id=c.id
		RETURNING j.id,j.user_id,COALESCE(j.canvas_id::text,''),j.status,j.test_mode,j.attempts,j.next_attempt_at,j.locked_until,j.completed_count,j.failed_count,j.credits_charged,j.credits_calculated,j.error,j.idempotency_key,j.created_at,j.updated_at`, now, limit, lease.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.BackgroundRemovalJob{}
	for rows.Next() {
		v, e := scanBackgroundRemovalJob(rows)
		if e != nil {
			return nil, e
		}
		v.Items, e = p.listBackgroundRemovalItems(ctx, v.ID)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (p *Postgres) UpdateBackgroundRemovalItem(ctx context.Context, item domain.BackgroundRemovalItem, node domain.ImageCanvasNode) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE background_removal_items SET status=$2,attempts=$3,credits_charged=$4,credits_calculated=$5,input_size=$6,result_size=$7,error=$8,updated_at=$9 WHERE id=$1`, item.ID, item.Status, item.Attempts, item.CreditsCharged, item.CreditsCalculated, item.InputSize, item.ResultSize, item.Error, item.UpdatedAt); err != nil {
		return err
	}
	if item.PlaceholderNodeID != "" {
		if _, err = tx.Exec(ctx, `UPDATE image_canvas_nodes SET status=$2,error=$3,updated_at=$4 WHERE id=$1`, item.PlaceholderNodeID, node.Status, node.Error, node.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) SaveBackgroundRemovalResult(ctx context.Context, item domain.BackgroundRemovalItem, asset domain.ImageAsset, node domain.ImageCanvasNode) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO image_assets(id,owner_id,project_id,object_key,mime_type,file_name,width,height,size_bytes,source,source_asset_id,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,'')::uuid,$12)`, asset.ID, asset.OwnerID, asset.ProjectID, asset.ObjectKey, asset.MIMEType, asset.FileName, asset.Width, asset.Height, asset.SizeBytes, asset.Source, asset.SourceAssetID, asset.CreatedAt); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE background_removal_items SET result_asset_id=$2,status='succeeded',attempts=$3,credits_charged=$4,credits_calculated=$5,input_size=$6,result_size=$7,error='',updated_at=$8 WHERE id=$1`, item.ID, asset.ID, item.Attempts, item.CreditsCharged, item.CreditsCalculated, item.InputSize, item.ResultSize, item.UpdatedAt); err != nil {
		return err
	}
	if item.PlaceholderNodeID != "" {
		if _, err = tx.Exec(ctx, `UPDATE image_canvas_nodes SET asset_id=$2,status='ready',error='',width=$3,height=$4,updated_at=$5 WHERE id=$1`, item.PlaceholderNodeID, asset.ID, node.Width, node.Height, node.UpdatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) UpdateBackgroundRemovalJob(ctx context.Context, v domain.BackgroundRemovalJob) error {
	tag, err := p.pool.Exec(ctx, `UPDATE background_removal_jobs SET status=$2,attempts=$3,next_attempt_at=$4,locked_until=$5,completed_count=$6,failed_count=$7,credits_charged=$8,credits_calculated=$9,error=$10,updated_at=$11 WHERE id=$1`, v.ID, v.Status, v.Attempts, v.NextAttemptAt, v.LockedUntil, v.CompletedCount, v.FailedCount, v.CreditsCharged, v.CreditsCalculated, v.Error, v.UpdatedAt)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) BackgroundRemovalStatistics(ctx context.Context, since time.Time) (domain.BackgroundRemovalStatistics, error) {
	var v domain.BackgroundRemovalStatistics
	err := p.pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE created_at>=date_trunc('day',now())),count(*) FILTER(WHERE status IN ('succeeded','partial')),count(*) FILTER(WHERE status='failed'),COALESCE(sum(completed_count+failed_count),0),COALESCE(sum(credits_charged),0),COALESCE(sum(credits_calculated),0) FROM background_removal_jobs WHERE created_at>=$1`, since).Scan(&v.TodayCalls, &v.ThirtyDaySucceeded, &v.ThirtyDayFailed, &v.ThirtyDayImages, &v.CreditsCharged, &v.CreditsCalculated)
	return v, err
}
