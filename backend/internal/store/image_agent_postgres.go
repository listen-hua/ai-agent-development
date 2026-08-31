package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
)

func (p *Postgres) ListImageRelays(ctx context.Context) ([]domain.ImageRelay, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,relay_key,name,base_url,enabled,timeout_seconds,allowed_output_hosts,
		encrypted_api_key,api_key_hint,COALESCE(created_by::text,''),COALESCE(updated_by::text,''),created_at,updated_at
		FROM image_relays ORDER BY created_at,relay_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ImageRelay{}
	for rows.Next() {
		value, scanErr := scanImageRelay(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (p *Postgres) GetImageRelay(ctx context.Context, id string) (domain.ImageRelay, error) {
	value, err := scanImageRelay(p.pool.QueryRow(ctx, `SELECT id,relay_key,name,base_url,enabled,timeout_seconds,allowed_output_hosts,
		encrypted_api_key,api_key_hint,COALESCE(created_by::text,''),COALESCE(updated_by::text,''),created_at,updated_at
		FROM image_relays WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	return value, err
}

func scanImageRelay(row rowScanner) (domain.ImageRelay, error) {
	var value domain.ImageRelay
	err := row.Scan(&value.ID, &value.RelayKey, &value.Name, &value.BaseURL, &value.Enabled, &value.TimeoutSeconds,
		&value.AllowedOutputHosts, &value.EncryptedAPIKey, &value.APIKeyHint, &value.CreatedBy, &value.UpdatedBy,
		&value.CreatedAt, &value.UpdatedAt)
	value.HasAPIKey = value.EncryptedAPIKey != ""
	return value, err
}

func (p *Postgres) UpsertImageRelay(ctx context.Context, value domain.ImageRelay) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO image_relays(id,relay_key,name,base_url,enabled,timeout_seconds,allowed_output_hosts,
		encrypted_api_key,api_key_hint,created_by,updated_by,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'' )::uuid,NULLIF($11,'')::uuid,$12,$13)
		ON CONFLICT(id) DO UPDATE SET relay_key=EXCLUDED.relay_key,name=EXCLUDED.name,base_url=EXCLUDED.base_url,
		enabled=EXCLUDED.enabled,timeout_seconds=EXCLUDED.timeout_seconds,allowed_output_hosts=EXCLUDED.allowed_output_hosts,
		encrypted_api_key=EXCLUDED.encrypted_api_key,api_key_hint=EXCLUDED.api_key_hint,
		updated_by=EXCLUDED.updated_by,updated_at=EXCLUDED.updated_at`,
		value.ID, value.RelayKey, value.Name, value.BaseURL, value.Enabled, value.TimeoutSeconds, value.AllowedOutputHosts,
		value.EncryptedAPIKey, value.APIKeyHint, value.CreatedBy, value.UpdatedBy, value.CreatedAt, value.UpdatedAt)
	return err
}

func (p *Postgres) ListImageModels(ctx context.Context, relayID string) ([]domain.ImageModel, error) {
	query := `SELECT id,relay_id,model_id,request_model_id,remote_endpoint_types,display_name,protocol,enabled,supports_reference,supports_reverse,
		supported_sizes,max_count,created_at,updated_at FROM image_models`
	args := []any{}
	if relayID != "" {
		query += ` WHERE relay_id=$1`
		args = append(args, relayID)
	}
	query += ` ORDER BY display_name,model_id`
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ImageModel{}
	for rows.Next() {
		var value domain.ImageModel
		if err = rows.Scan(&value.ID, &value.RelayID, &value.ModelID, &value.RequestModelID, &value.RemoteEndpointTypes, &value.DisplayName, &value.Protocol, &value.Enabled,
			&value.SupportsReference, &value.SupportsReverse, &value.SupportedSizes, &value.MaxCount, &value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (p *Postgres) GetImageModel(ctx context.Context, id string) (domain.ImageModel, error) {
	var value domain.ImageModel
	err := p.pool.QueryRow(ctx, `SELECT id,relay_id,model_id,request_model_id,remote_endpoint_types,display_name,protocol,enabled,supports_reference,supports_reverse,
		supported_sizes,max_count,created_at,updated_at FROM image_models WHERE id=$1`, id).
		Scan(&value.ID, &value.RelayID, &value.ModelID, &value.RequestModelID, &value.RemoteEndpointTypes, &value.DisplayName, &value.Protocol, &value.Enabled,
			&value.SupportsReference, &value.SupportsReverse, &value.SupportedSizes, &value.MaxCount, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	return value, err
}

func (p *Postgres) UpsertImageModels(ctx context.Context, values []domain.ImageModel) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, value := range values {
		_, err = tx.Exec(ctx, `INSERT INTO image_models(id,relay_id,model_id,request_model_id,remote_endpoint_types,display_name,protocol,enabled,supports_reference,
			supports_reverse,supported_sizes,max_count,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
			ON CONFLICT(relay_id,model_id) DO UPDATE SET request_model_id=EXCLUDED.request_model_id,
			remote_endpoint_types=EXCLUDED.remote_endpoint_types,updated_at=EXCLUDED.updated_at`,
			value.ID, value.RelayID, value.ModelID, value.RequestModelID, value.RemoteEndpointTypes, value.DisplayName, value.Protocol, value.Enabled,
			value.SupportsReference, value.SupportsReverse, value.SupportedSizes, value.MaxCount, value.CreatedAt, value.UpdatedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) UpdateImageModel(ctx context.Context, value domain.ImageModel) error {
	tag, err := p.pool.Exec(ctx, `UPDATE image_models SET request_model_id=$2,remote_endpoint_types=$3,display_name=$4,protocol=$5,enabled=$6,supports_reference=$7,
		supports_reverse=$8,supported_sizes=$9,max_count=$10,updated_at=$11 WHERE id=$1`,
		value.ID, value.RequestModelID, value.RemoteEndpointTypes,
		value.DisplayName, value.Protocol, value.Enabled, value.SupportsReference, value.SupportsReverse,
		value.SupportedSizes, value.MaxCount, value.UpdatedAt)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) ListImageProjects(ctx context.Context) ([]domain.ImageProject, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,project_key,name,description,acl,enabled,COALESCE(created_by::text,''),
		COALESCE(updated_by::text,''),created_at,updated_at FROM image_projects ORDER BY name,created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ImageProject{}
	for rows.Next() {
		value, scanErr := scanImageProject(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (p *Postgres) GetImageProject(ctx context.Context, id string) (domain.ImageProject, error) {
	value, err := scanImageProject(p.pool.QueryRow(ctx, `SELECT id,project_key,name,description,acl,enabled,
		COALESCE(created_by::text,''),COALESCE(updated_by::text,''),created_at,updated_at FROM image_projects WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	return value, err
}

func scanImageProject(row rowScanner) (domain.ImageProject, error) {
	var value domain.ImageProject
	var acl []byte
	err := row.Scan(&value.ID, &value.ProjectKey, &value.Name, &value.Description, &acl, &value.Enabled,
		&value.CreatedBy, &value.UpdatedBy, &value.CreatedAt, &value.UpdatedAt)
	if err == nil {
		err = json.Unmarshal(acl, &value.ACL)
	}
	return value, err
}

func (p *Postgres) UpsertImageProject(ctx context.Context, value domain.ImageProject) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO image_projects(id,project_key,name,description,acl,enabled,created_by,updated_by,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid,NULLIF($8,'')::uuid,$9,$10)
		ON CONFLICT(id) DO UPDATE SET project_key=EXCLUDED.project_key,name=EXCLUDED.name,description=EXCLUDED.description,
		acl=EXCLUDED.acl,enabled=EXCLUDED.enabled,updated_by=EXCLUDED.updated_by,updated_at=EXCLUDED.updated_at`,
		value.ID, value.ProjectKey, value.Name, value.Description, mustJSON(value.ACL), value.Enabled,
		value.CreatedBy, value.UpdatedBy, value.CreatedAt, value.UpdatedAt)
	return err
}

func (p *Postgres) DeleteImageProject(ctx context.Context, id string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var lockedID string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM image_projects WHERE id=$1 FOR UPDATE`, id).Scan(&lockedID); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	var dependencies int
	if err = tx.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM image_assets WHERE project_id=$1) +
		(SELECT count(*) FROM image_jobs WHERE project_id=$1) +
		(SELECT count(*) FROM image_canvases WHERE project_id=$1)`, id).Scan(&dependencies); err != nil {
		return err
	}
	if dependencies > 0 {
		return ErrConflict
	}
	if _, err = tx.Exec(ctx, `DELETE FROM image_prompt_actions
		WHERE project_ids @> ARRAY[$1::uuid] AND cardinality(project_ids)=1`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE image_prompt_actions
		SET project_ids=array_remove(project_ids,$1::uuid),
			project_id=(array_remove(project_ids,$1::uuid))[1],
			updated_at=now()
		WHERE project_ids @> ARRAY[$1::uuid]`, id); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM image_projects WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (p *Postgres) ListImagePromptActions(ctx context.Context, projectID string) ([]domain.ImagePromptAction, error) {
	query := `SELECT id,action_key,name,prompt_template,COALESCE(project_id::text,''),COALESCE(project_ids::text[],ARRAY[]::text[]),enabled,sort_order,
		preview_object_key,preview_mime_type,preview_size_bytes,preview_width,preview_height,
		COALESCE(created_by::text,''),COALESCE(updated_by::text,''),created_at,updated_at FROM image_prompt_actions`
	args := []any{}
	if projectID != "" {
		query += ` WHERE (project_id IS NULL AND cardinality(project_ids)=0) OR project_id=$1 OR project_ids @> ARRAY[$1::uuid]`
		args = append(args, projectID)
	}
	query += ` ORDER BY sort_order,name`
	rows, err := p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ImagePromptAction{}
	for rows.Next() {
		var value domain.ImagePromptAction
		if err = rows.Scan(&value.ID, &value.ActionKey, &value.Name, &value.PromptTemplate, &value.ProjectID, &value.ProjectIDs,
			&value.Enabled, &value.SortOrder, &value.PreviewObjectKey, &value.PreviewMIMEType,
			&value.PreviewSizeBytes, &value.PreviewWidth, &value.PreviewHeight,
			&value.CreatedBy, &value.UpdatedBy, &value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		value.HasPreview = value.PreviewObjectKey != ""
		result = append(result, value)
	}
	return result, rows.Err()
}

func (p *Postgres) GetImagePromptAction(ctx context.Context, id string) (domain.ImagePromptAction, error) {
	var value domain.ImagePromptAction
	err := p.pool.QueryRow(ctx, `SELECT id,action_key,name,prompt_template,COALESCE(project_id::text,''),COALESCE(project_ids::text[],ARRAY[]::text[]),enabled,sort_order,
		preview_object_key,preview_mime_type,preview_size_bytes,preview_width,preview_height,
		COALESCE(created_by::text,''),COALESCE(updated_by::text,''),created_at,updated_at FROM image_prompt_actions WHERE id=$1`, id).
		Scan(&value.ID, &value.ActionKey, &value.Name, &value.PromptTemplate, &value.ProjectID, &value.ProjectIDs, &value.Enabled,
			&value.SortOrder, &value.PreviewObjectKey, &value.PreviewMIMEType, &value.PreviewSizeBytes,
			&value.PreviewWidth, &value.PreviewHeight, &value.CreatedBy, &value.UpdatedBy, &value.CreatedAt, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	value.HasPreview = value.PreviewObjectKey != ""
	return value, err
}

func (p *Postgres) UpsertImagePromptAction(ctx context.Context, value domain.ImagePromptAction) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO image_prompt_actions(id,action_key,name,prompt_template,project_id,project_ids,enabled,sort_order,
		created_by,updated_by,created_at,updated_at)
		VALUES($1,$2,$3,$4,NULLIF($5,'')::uuid,$6::uuid[],$7,$8,NULLIF($9,'')::uuid,NULLIF($10,'')::uuid,$11,$12)
		ON CONFLICT(id) DO UPDATE SET action_key=EXCLUDED.action_key,name=EXCLUDED.name,prompt_template=EXCLUDED.prompt_template,
		project_id=EXCLUDED.project_id,project_ids=EXCLUDED.project_ids,enabled=EXCLUDED.enabled,sort_order=EXCLUDED.sort_order,
		updated_by=EXCLUDED.updated_by,updated_at=EXCLUDED.updated_at`,
		value.ID, value.ActionKey, value.Name, value.PromptTemplate, value.ProjectID, value.ProjectIDs, value.Enabled, value.SortOrder,
		value.CreatedBy, value.UpdatedBy, value.CreatedAt, value.UpdatedAt)
	return err
}

func (p *Postgres) UpdateImagePromptActionPreview(ctx context.Context, value domain.ImagePromptAction) error {
	tag, err := p.pool.Exec(ctx, `UPDATE image_prompt_actions SET preview_object_key=$2,preview_mime_type=$3,
		preview_size_bytes=$4,preview_width=$5,preview_height=$6,updated_by=NULLIF($7,'')::uuid,updated_at=$8
		WHERE id=$1`, value.ID, value.PreviewObjectKey, value.PreviewMIMEType, value.PreviewSizeBytes,
		value.PreviewWidth, value.PreviewHeight, value.UpdatedBy, value.UpdatedAt)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) DeleteImagePromptAction(ctx context.Context, id string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM image_prompt_actions WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) ListImageCanvases(ctx context.Context, userID string, deleted bool) ([]domain.ImageCanvas, error) {
	operator := "IS NULL"
	if deleted {
		operator = "IS NOT NULL"
	}
	rows, err := p.pool.Query(ctx, `SELECT c.id,c.user_id,COALESCE(c.project_id::text,''),c.name,c.viewport,c.version,c.deleted_at,c.created_at,c.updated_at,
		(SELECT count(*) FROM image_canvas_nodes n WHERE n.canvas_id=c.id),
		COALESCE((SELECT n.asset_id::text FROM image_canvas_nodes n WHERE n.canvas_id=c.id AND n.status='ready' AND n.asset_id IS NOT NULL ORDER BY n.z_index,n.created_at,n.id LIMIT 1),'')
		FROM image_canvases c WHERE c.user_id=$1 AND c.deleted_at `+operator+` ORDER BY c.updated_at DESC,c.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ImageCanvas{}
	for rows.Next() {
		value, scanErr := scanImageCanvas(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		value.Nodes = []domain.ImageCanvasNode{}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (p *Postgres) CreateImageCanvas(ctx context.Context, value domain.ImageCanvas) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO image_canvases(id,user_id,project_id,name,viewport,version,created_at,updated_at)
		VALUES($1,$2,NULL,$3,$4,$5,$6,$7)`, value.ID, value.UserID, value.Name, mustJSON(value.Viewport), value.Version, value.CreatedAt, value.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}

func (p *Postgres) GetImageCanvas(ctx context.Context, id string) (domain.ImageCanvas, error) {
	value, err := scanImageCanvas(p.pool.QueryRow(ctx, `SELECT c.id,c.user_id,COALESCE(c.project_id::text,''),c.name,c.viewport,c.version,c.deleted_at,c.created_at,c.updated_at,
		(SELECT count(*) FROM image_canvas_nodes n WHERE n.canvas_id=c.id),
		COALESCE((SELECT n.asset_id::text FROM image_canvas_nodes n WHERE n.canvas_id=c.id AND n.status='ready' AND n.asset_id IS NOT NULL ORDER BY n.z_index,n.created_at,n.id LIMIT 1),'')
		FROM image_canvases c WHERE c.id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	if err != nil {
		return value, err
	}
	value.Nodes, err = p.listImageCanvasNodes(ctx, value.ID)
	return value, err
}

func scanImageCanvas(row rowScanner) (domain.ImageCanvas, error) {
	var value domain.ImageCanvas
	var viewport []byte
	err := row.Scan(&value.ID, &value.UserID, &value.ProjectID, &value.Name, &viewport, &value.Version, &value.DeletedAt,
		&value.CreatedAt, &value.UpdatedAt, &value.NodeCount, &value.PreviewAssetID)
	if err == nil {
		err = json.Unmarshal(viewport, &value.Viewport)
	}
	return value, err
}

func (p *Postgres) SoftDeleteImageCanvas(ctx context.Context, id, userID string, now time.Time) error {
	tag, err := p.pool.Exec(ctx, `UPDATE image_canvases SET deleted_at=$3,updated_at=$3 WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`, id, userID, now)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) RestoreImageCanvas(ctx context.Context, id, userID string, now time.Time) (domain.ImageCanvas, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.ImageCanvas{}, err
	}
	defer tx.Rollback(ctx)
	var original string
	if err = tx.QueryRow(ctx, `SELECT name FROM image_canvases WHERE id=$1 AND user_id=$2 AND deleted_at IS NOT NULL FOR UPDATE`, id, userID).Scan(&original); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ImageCanvas{}, ErrNotFound
		}
		return domain.ImageCanvas{}, err
	}
	name := original
	for suffix := 0; ; suffix++ {
		if suffix > 0 {
			base := []rune(strings.TrimSpace(original))
			if len(base) > 65 {
				base = base[:65]
			}
			name = string(base) + "（恢复）"
			if suffix > 1 {
				name = fmt.Sprintf("%s（恢复%d）", string(base), suffix)
			}
		}
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM image_canvases WHERE user_id=$1 AND deleted_at IS NULL AND lower(name)=lower($2))`, userID, name).Scan(&exists); err != nil {
			return domain.ImageCanvas{}, err
		}
		if !exists {
			break
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE image_canvases SET name=$3,deleted_at=NULL,updated_at=$4 WHERE id=$1 AND user_id=$2`, id, userID, name, now); err != nil {
		return domain.ImageCanvas{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.ImageCanvas{}, err
	}
	return p.GetImageCanvas(ctx, id)
}

func (p *Postgres) CleanupDeletedImageCanvases(ctx context.Context, before time.Time) (int64, error) {
	tag, err := p.pool.Exec(ctx, `DELETE FROM image_canvases c WHERE c.deleted_at IS NOT NULL AND c.deleted_at<$1
		AND NOT EXISTS (SELECT 1 FROM image_jobs j WHERE j.canvas_id=c.id AND j.status IN ('pending','running','retry'))`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (p *Postgres) GetOrCreateImageCanvas(ctx context.Context, userID, projectID string) (domain.ImageCanvas, error) {
	project, err := p.GetImageProject(ctx, projectID)
	if err != nil {
		return domain.ImageCanvas{}, err
	}
	baseName := []rune(strings.TrimSpace(project.Name))
	if len(baseName) > 68 {
		baseName = baseName[:68]
	}
	legacyName := string(baseName) + "-" + projectID[:min(8, len(projectID))]
	_, err = p.pool.Exec(ctx, `INSERT INTO image_canvases(id,user_id,project_id,name) VALUES($1,$2,$3,$4)
		ON CONFLICT(user_id,project_id) WHERE project_id IS NOT NULL DO NOTHING`, ids.New("canvas"), userID, projectID, legacyName)
	if err != nil {
		return domain.ImageCanvas{}, err
	}
	var id string
	err = p.pool.QueryRow(ctx, `SELECT id FROM image_canvases WHERE user_id=$1 AND project_id=$2 AND deleted_at IS NULL`, userID, projectID).Scan(&id)
	if err != nil {
		return domain.ImageCanvas{}, err
	}
	return p.GetImageCanvas(ctx, id)
}

func (p *Postgres) listImageCanvasNodes(ctx context.Context, canvasID string) ([]domain.ImageCanvasNode, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,canvas_id,COALESCE(asset_id::text,''),COALESCE(job_id::text,''),
		COALESCE(background_removal_job_id::text,''),COALESCE(source_node_id::text,''),output_index,
		status,x,y,width,height,z_index,error,requested_size,actual_width,actual_height,resolution_warning,
		generation_relay_name,generation_model_name,generation_model_key,created_at,updated_at
		FROM image_canvas_nodes WHERE canvas_id=$1 ORDER BY z_index,created_at`, canvasID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ImageCanvasNode{}
	for rows.Next() {
		var value domain.ImageCanvasNode
		if err = rows.Scan(&value.ID, &value.CanvasID, &value.AssetID, &value.JobID, &value.BackgroundRemovalJobID,
			&value.SourceNodeID, &value.OutputIndex, &value.Status,
			&value.X, &value.Y, &value.Width, &value.Height, &value.ZIndex, &value.Error, &value.RequestedSize,
			&value.ActualWidth, &value.ActualHeight, &value.ResolutionWarning, &value.GenerationRelayName,
			&value.GenerationModelName, &value.GenerationModelKey, &value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (p *Postgres) UpdateImageCanvas(ctx context.Context, value domain.ImageCanvas, expectedVersion int64) (domain.ImageCanvas, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return value, err
	}
	defer tx.Rollback(ctx)
	var version int64
	err = tx.QueryRow(ctx, `UPDATE image_canvases SET viewport=$3,version=version+1,updated_at=now()
		WHERE id=$1 AND version=$2 AND deleted_at IS NULL RETURNING version`, value.ID, expectedVersion, mustJSON(value.Viewport)).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrConflict
	}
	if err != nil {
		return value, err
	}
	for _, node := range value.Nodes {
		tag, updateErr := tx.Exec(ctx, `UPDATE image_canvas_nodes SET x=$3,y=$4,width=$5,height=$6,z_index=$7,updated_at=now()
			WHERE id=$1 AND canvas_id=$2`, node.ID, value.ID, node.X, node.Y, node.Width, node.Height, node.ZIndex)
		if updateErr != nil {
			return value, updateErr
		}
		if tag.RowsAffected() == 0 {
			return value, ErrForbidden
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return value, err
	}
	value.Version = version
	value.UpdatedAt = time.Now()
	return value, nil
}

func (p *Postgres) ImportImageCanvasAsset(ctx context.Context, canvas domain.ImageCanvas, expectedVersion int64, asset domain.ImageAsset, node domain.ImageCanvasNode) (domain.ImageCanvas, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return canvas, err
	}
	defer tx.Rollback(ctx)
	var version int64
	err = tx.QueryRow(ctx, `UPDATE image_canvases SET version=version+1,updated_at=now()
		WHERE id=$1 AND user_id=$2 AND version=$3 AND deleted_at IS NULL RETURNING version`,
		canvas.ID, canvas.UserID, expectedVersion).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return canvas, ErrConflict
	}
	if err != nil {
		return canvas, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO image_assets(id,owner_id,project_id,object_key,mime_type,file_name,width,height,size_bytes,source,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		asset.ID, asset.OwnerID, asset.ProjectID, asset.ObjectKey, asset.MIMEType, asset.FileName,
		asset.Width, asset.Height, asset.SizeBytes, asset.Source, asset.CreatedAt); err != nil {
		return canvas, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO image_canvas_nodes(id,canvas_id,asset_id,output_index,status,x,y,width,height,z_index,error,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,
			(SELECT COALESCE(MAX(z_index),0)+1 FROM image_canvas_nodes WHERE canvas_id=$2),$10,$11,$12)
		RETURNING z_index`,
		node.ID, canvas.ID, asset.ID, node.OutputIndex, node.Status, node.X, node.Y, node.Width, node.Height,
		node.Error, node.CreatedAt, node.UpdatedAt).Scan(&node.ZIndex)
	if err != nil {
		return canvas, err
	}
	if err = tx.Commit(ctx); err != nil {
		return canvas, err
	}
	canvas.Version = version
	canvas.UpdatedAt = time.Now()
	canvas.Nodes = append(canvas.Nodes, node)
	return canvas, nil
}

func (p *Postgres) DeleteImageCanvasNode(ctx context.Context, canvas domain.ImageCanvas, nodeID string, expectedVersion int64) (domain.ImageCanvas, error) {
	return p.DeleteImageCanvasNodes(ctx, canvas, []string{nodeID}, expectedVersion)
}

func (p *Postgres) DeleteImageCanvasNodes(ctx context.Context, canvas domain.ImageCanvas, nodeIDs []string, expectedVersion int64) (domain.ImageCanvas, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return canvas, err
	}
	defer tx.Rollback(ctx)
	var version int64
	err = tx.QueryRow(ctx, `UPDATE image_canvases SET version=version+1,updated_at=now()
		WHERE id=$1 AND user_id=$2 AND version=$3 AND deleted_at IS NULL RETURNING version`,
		canvas.ID, canvas.UserID, expectedVersion).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return canvas, ErrConflict
	}
	if err != nil {
		return canvas, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM image_canvas_nodes WHERE canvas_id=$1 AND id=ANY($2::uuid[])`, canvas.ID, nodeIDs)
	if err != nil {
		return canvas, err
	}
	if tag.RowsAffected() != int64(len(nodeIDs)) {
		return canvas, ErrNotFound
	}
	if err = tx.Commit(ctx); err != nil {
		return canvas, err
	}
	canvas.Version = version
	canvas.UpdatedAt = time.Now()
	deleting := make(map[string]bool, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		deleting[nodeID] = true
	}
	nodes := make([]domain.ImageCanvasNode, 0, len(canvas.Nodes)-len(nodeIDs))
	for _, node := range canvas.Nodes {
		if !deleting[node.ID] {
			nodes = append(nodes, node)
		}
	}
	canvas.Nodes = nodes
	return canvas, nil
}

func (p *Postgres) CreateImageAsset(ctx context.Context, value domain.ImageAsset) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO image_assets(id,owner_id,project_id,object_key,mime_type,file_name,width,height,size_bytes,source,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		value.ID, value.OwnerID, value.ProjectID, value.ObjectKey, value.MIMEType, value.FileName,
		value.Width, value.Height, value.SizeBytes, value.Source, value.CreatedAt)
	return err
}

func (p *Postgres) GetImageAsset(ctx context.Context, id string) (domain.ImageAsset, error) {
	var value domain.ImageAsset
	err := p.pool.QueryRow(ctx, `SELECT id,owner_id,project_id,object_key,mime_type,file_name,width,height,size_bytes,source,
		COALESCE(source_asset_id::text,''),created_at
		FROM image_assets WHERE id=$1`, id).Scan(&value.ID, &value.OwnerID, &value.ProjectID, &value.ObjectKey,
		&value.MIMEType, &value.FileName, &value.Width, &value.Height, &value.SizeBytes, &value.Source, &value.SourceAssetID, &value.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	return value, err
}

func (p *Postgres) CreateImageJob(ctx context.Context, value domain.ImageJob, nodes []domain.ImageCanvasNode, expectedCanvasVersion int64) (domain.ImageJob, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return value, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `INSERT INTO image_jobs(id,user_id,project_id,canvas_id,relay_id,model_id,kind,prompt,
		aspect_ratio,image_size,count,reference_asset_ids,status,attempts,next_attempt_at,completed_count,reversed_prompt,
		error,idempotency_key,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
		ON CONFLICT(user_id,idempotency_key) DO NOTHING`,
		value.ID, value.UserID, value.ProjectID, value.CanvasID, value.RelayID, value.ModelID, value.Kind, value.Prompt,
		value.AspectRatio, value.ImageSize, value.Count, value.ReferenceAssetIDs, value.Status, value.Attempts, value.NextAttemptAt,
		value.CompletedCount, value.ReversedPrompt, value.Error, value.IdempotencyKey, value.CreatedAt, value.UpdatedAt)
	if err != nil {
		return value, err
	}
	if tag.RowsAffected() == 0 {
		tx.Rollback(ctx)
		return p.getImageJobByIdempotency(ctx, value.UserID, value.IdempotencyKey)
	}
	if len(nodes) > 0 {
		tag, err = tx.Exec(ctx, `UPDATE image_canvases SET version=version+1,updated_at=now()
			WHERE id=$1 AND version=$2 AND user_id=$3 AND deleted_at IS NULL`, value.CanvasID, expectedCanvasVersion, value.UserID)
		if err != nil {
			return value, err
		}
		if tag.RowsAffected() == 0 {
			return value, ErrConflict
		}
	}
	for index := 0; index < value.Count; index++ {
		if _, err = tx.Exec(ctx, `INSERT INTO image_job_outputs(id,job_id,output_index,status,requested_size,created_at,updated_at)
			VALUES($1,$2,$3,'pending',$4,$5,$5)`, ids.New("output"), value.ID, index, value.ImageSize, value.CreatedAt); err != nil {
			return value, err
		}
	}
	for _, node := range nodes {
		if _, err = tx.Exec(ctx, `INSERT INTO image_canvas_nodes(id,canvas_id,job_id,output_index,status,x,y,width,height,z_index,error,requested_size,
			generation_relay_name,generation_model_name,generation_model_key,created_at,updated_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
			node.ID, node.CanvasID, node.JobID, node.OutputIndex, node.Status, node.X, node.Y, node.Width, node.Height,
			node.ZIndex, node.Error, node.RequestedSize, node.GenerationRelayName, node.GenerationModelName,
			node.GenerationModelKey, node.CreatedAt, node.UpdatedAt); err != nil {
			return value, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return value, err
	}
	return p.GetImageJob(ctx, value.ID)
}

func (p *Postgres) getImageJobByIdempotency(ctx context.Context, userID, key string) (domain.ImageJob, error) {
	var id string
	err := p.pool.QueryRow(ctx, `SELECT id FROM image_jobs WHERE user_id=$1 AND idempotency_key=$2`, userID, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ImageJob{}, ErrNotFound
	}
	if err != nil {
		return domain.ImageJob{}, err
	}
	return p.GetImageJob(ctx, id)
}

func (p *Postgres) GetImageJob(ctx context.Context, id string) (domain.ImageJob, error) {
	value, err := scanImageJob(p.pool.QueryRow(ctx, imageJobSelect+` WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	if err != nil {
		return value, err
	}
	value.Outputs, err = p.listImageJobOutputs(ctx, value.ID)
	return value, err
}

const imageJobSelect = `SELECT id,user_id,project_id,COALESCE(canvas_id::text,''),relay_id,model_id,kind,prompt,aspect_ratio,image_size,
	count,reference_asset_ids,status,attempts,next_attempt_at,locked_until,completed_count,reversed_prompt,error,
	idempotency_key,created_at,updated_at FROM image_jobs`

func scanImageJob(row rowScanner) (domain.ImageJob, error) {
	var value domain.ImageJob
	err := row.Scan(&value.ID, &value.UserID, &value.ProjectID, &value.CanvasID, &value.RelayID, &value.ModelID,
		&value.Kind, &value.Prompt, &value.AspectRatio, &value.ImageSize, &value.Count, &value.ReferenceAssetIDs,
		&value.Status, &value.Attempts, &value.NextAttemptAt, &value.LockedUntil, &value.CompletedCount,
		&value.ReversedPrompt, &value.Error, &value.IdempotencyKey, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func (p *Postgres) listImageJobOutputs(ctx context.Context, jobID string) ([]domain.ImageJobOutput, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,job_id,output_index,COALESCE(asset_id::text,''),status,error,attempts,
		requested_size,actual_width,actual_height,resolution_warning,created_at,updated_at
		FROM image_job_outputs WHERE job_id=$1 ORDER BY output_index`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ImageJobOutput{}
	for rows.Next() {
		var value domain.ImageJobOutput
		if err = rows.Scan(&value.ID, &value.JobID, &value.OutputIndex, &value.AssetID, &value.Status, &value.Error,
			&value.Attempts, &value.RequestedSize, &value.ActualWidth, &value.ActualHeight,
			&value.ResolutionWarning, &value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (p *Postgres) ListImageJobs(ctx context.Context, userID, projectID string, limit int) ([]domain.ImageJob, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := p.pool.Query(ctx, imageJobSelect+` WHERE user_id=$1 AND ($2='' OR project_id::text=$2)
		ORDER BY created_at DESC LIMIT $3`, userID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ImageJob{}
	for rows.Next() {
		value, scanErr := scanImageJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		value.Outputs, scanErr = p.listImageJobOutputs(ctx, value.ID)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (p *Postgres) ClaimImageJobs(ctx context.Context, now time.Time, lease time.Duration, limit int) ([]domain.ImageJob, error) {
	if limit <= 0 {
		limit = 4
	}
	rows, err := p.pool.Query(ctx, `WITH candidates AS (
		SELECT id FROM image_jobs WHERE status IN ('pending','retry') AND next_attempt_at<=$1
			AND (locked_until IS NULL OR locked_until<$1) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT $2
	) UPDATE image_jobs j SET status='running',attempts=j.attempts+1,locked_until=$1+$3::interval,updated_at=$1
	FROM candidates c WHERE j.id=c.id
	RETURNING j.id,j.user_id,j.project_id,COALESCE(j.canvas_id::text,''),j.relay_id,j.model_id,j.kind,j.prompt,j.aspect_ratio,j.image_size,
		j.count,j.reference_asset_ids,j.status,j.attempts,j.next_attempt_at,j.locked_until,j.completed_count,j.reversed_prompt,
		j.error,j.idempotency_key,j.created_at,j.updated_at`, now, limit, lease.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []domain.ImageJob{}
	for rows.Next() {
		value, scanErr := scanImageJob(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		value.Outputs, scanErr = p.listImageJobOutputs(ctx, value.ID)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (p *Postgres) UpdateImageJob(ctx context.Context, value domain.ImageJob) error {
	tag, err := p.pool.Exec(ctx, `UPDATE image_jobs SET status=$2,attempts=$3,next_attempt_at=$4,locked_until=$5,
		completed_count=$6,reversed_prompt=$7,error=$8,updated_at=$9 WHERE id=$1`,
		value.ID, value.Status, value.Attempts, value.NextAttemptAt, value.LockedUntil, value.CompletedCount,
		value.ReversedPrompt, value.Error, value.UpdatedAt)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) SaveImageJobOutput(ctx context.Context, output domain.ImageJobOutput, asset domain.ImageAsset, node domain.ImageCanvasNode) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO image_assets(id,owner_id,project_id,object_key,mime_type,file_name,width,height,size_bytes,source,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, asset.ID, asset.OwnerID, asset.ProjectID, asset.ObjectKey,
		asset.MIMEType, asset.FileName, asset.Width, asset.Height, asset.SizeBytes, asset.Source, asset.CreatedAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE image_job_outputs SET asset_id=$2,status='succeeded',error='',attempts=$3,
		requested_size=$4,actual_width=$5,actual_height=$6,resolution_warning=$7,updated_at=$8 WHERE id=$1`,
		output.ID, asset.ID, output.Attempts, output.RequestedSize, output.ActualWidth, output.ActualHeight,
		output.ResolutionWarning, output.UpdatedAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE image_canvas_nodes SET asset_id=$3,status='ready',error='',width=$4,height=$5,
		requested_size=$6,actual_width=$7,actual_height=$8,resolution_warning=$9,updated_at=$10
		WHERE job_id=$1 AND output_index=$2`, output.JobID, output.OutputIndex, asset.ID, node.Width, node.Height,
		node.RequestedSize, node.ActualWidth, node.ActualHeight, node.ResolutionWarning, node.UpdatedAt)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) UpdateImageJobOutputFailure(ctx context.Context, output domain.ImageJobOutput, node domain.ImageCanvasNode) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE image_job_outputs SET status='failed',error=$2,attempts=$3,updated_at=$4 WHERE id=$1`,
		output.ID, output.Error, output.Attempts, output.UpdatedAt); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE image_canvas_nodes SET status='failed',error=$3,updated_at=$4
		WHERE job_id=$1 AND output_index=$2`, output.JobID, output.OutputIndex, node.Error, node.UpdatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (p *Postgres) CleanupImageJobLogs(ctx context.Context, before time.Time) error {
	_, err := p.pool.Exec(ctx, `UPDATE image_jobs SET prompt='',error='',idempotency_key='expired-'||id::text
		WHERE created_at<$1 AND status IN ('partial','succeeded','failed','cancelled')`, before)
	return err
}
