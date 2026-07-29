package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
)

type Postgres struct{ pool *pgxpool.Pool }

func OpenPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}
func (p *Postgres) Close() { p.pool.Close() }
func (p *Postgres) EnsureSeed(ctx context.Context, cfg domain.AgentConfig) error {
	admin := domain.User{ID: DemoAdminID, FeishuOpenID: "ou_demo_admin", Name: "演示管理员", DepartmentIDs: []string{"dept_admin"}, JobTitle: "系统管理员", Status: "active", Roles: []domain.Role{domain.RoleEmployee, domain.RoleSuperAdmin}}
	employee := domain.User{ID: DemoEmployeeID, FeishuOpenID: "ou_demo_employee", Name: "演示员工", DepartmentIDs: []string{"dept_product"}, JobTitle: "产品经理", Status: "active", Roles: []domain.Role{domain.RoleEmployee}}
	if _, err := p.UpsertUser(ctx, admin); err != nil {
		return err
	}
	if _, err := p.UpsertUser(ctx, employee); err != nil {
		return err
	}
	if _, err := p.PublishedConfig(ctx); err == ErrNotFound {
		now := time.Now()
		return p.SaveConfig(ctx, domain.AgentConfigVersion{ID: ids.New("cfg"), Version: 1, Status: "published", Config: cfg, CreatedBy: admin.ID, CreatedAt: now, PublishedAt: &now})
	}
	return nil
}

func (p *Postgres) UpsertUser(ctx context.Context, u domain.User) (domain.User, error) {
	if u.ID == "" {
		u.ID = ids.New("usr")
	}
	if u.DepartmentIDs == nil {
		u.DepartmentIDs = []string{}
	}
	if u.Status == "" {
		u.Status = "active"
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return u, err
	}
	defer tx.Rollback(ctx)
	orgSynced := u.OrganizationSyncedAt != nil
	err = tx.QueryRow(ctx, `INSERT INTO users(id,feishu_open_id,feishu_user_id,iam_user_id,name,avatar_url,department_ids,job_title,job_level_id,job_family_id,employee_type,status,organization_synced_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT(feishu_open_id) DO UPDATE SET
			name=excluded.name,
			avatar_url=CASE WHEN excluded.avatar_url<>'' THEN excluded.avatar_url ELSE users.avatar_url END,
			feishu_user_id=COALESCE(NULLIF(excluded.feishu_user_id,''),users.feishu_user_id),
			iam_user_id=COALESCE(excluded.iam_user_id,users.iam_user_id),
			department_ids=CASE WHEN $14 THEN excluded.department_ids ELSE users.department_ids END,
			job_title=CASE WHEN $14 THEN excluded.job_title ELSE users.job_title END,
			job_level_id=CASE WHEN $14 THEN excluded.job_level_id ELSE users.job_level_id END,
			job_family_id=CASE WHEN $14 THEN excluded.job_family_id ELSE users.job_family_id END,
			employee_type=CASE WHEN $14 THEN excluded.employee_type ELSE users.employee_type END,
			status=CASE WHEN $14 THEN excluded.status ELSE users.status END,
			organization_synced_at=CASE WHEN $14 THEN excluded.organization_synced_at ELSE users.organization_synced_at END,
			updated_at=now()
		RETURNING id`, u.ID, u.FeishuOpenID, u.FeishuUserID, u.IAMUserID, u.Name, u.AvatarURL, u.DepartmentIDs, u.JobTitle, u.JobLevelID, u.JobFamilyID, u.EmployeeType, u.Status, u.OrganizationSyncedAt, orgSynced).Scan(&u.ID)
	if err != nil {
		return u, err
	}
	if len(u.Roles) == 0 {
		u.Roles = []domain.Role{domain.RoleEmployee}
	}
	for _, role := range u.Roles {
		if _, err = tx.Exec(ctx, `INSERT INTO role_assignments(user_id,role) VALUES($1,$2) ON CONFLICT DO NOTHING`, u.ID, role); err != nil {
			return u, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return u, err
	}
	return p.GetUser(ctx, u.ID)
}

func (p *Postgres) UpsertIAMUser(ctx context.Context, u domain.User) (domain.User, error) {
	if u.IAMUserID == nil || *u.IAMUserID <= 0 || strings.TrimSpace(u.FeishuOpenID) == "" || strings.TrimSpace(u.FeishuUserID) == "" {
		return domain.User{}, ErrConflict
	}
	if u.ID == "" {
		u.ID = ids.New("usr")
	}
	if u.DepartmentIDs == nil {
		u.DepartmentIDs = []string{}
	}
	if u.Status == "" {
		u.Status = "active"
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback(ctx)

	var existingID, existingFeishuUserID string
	var existingIAMUserID *int64
	err = tx.QueryRow(ctx, `SELECT id,iam_user_id,COALESCE(feishu_user_id,'') FROM users WHERE feishu_open_id=$1 FOR UPDATE`, u.FeishuOpenID).
		Scan(&existingID, &existingIAMUserID, &existingFeishuUserID)
	switch {
	case err == nil:
		if existingIAMUserID != nil && *existingIAMUserID != *u.IAMUserID {
			return domain.User{}, ErrConflict
		}
		if existingFeishuUserID != "" && existingFeishuUserID != u.FeishuUserID {
			return domain.User{}, ErrConflict
		}
		u.ID = existingID
	case errors.Is(err, pgx.ErrNoRows):
		var conflicting bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM users
			WHERE iam_user_id=$1 OR (feishu_user_id IS NOT NULL AND feishu_user_id=$2)
		)`, *u.IAMUserID, u.FeishuUserID).Scan(&conflicting); err != nil {
			return domain.User{}, err
		}
		if conflicting {
			return domain.User{}, ErrConflict
		}
	default:
		return domain.User{}, err
	}

	if existingID == "" {
		_, err = tx.Exec(ctx, `INSERT INTO users(id,feishu_open_id,feishu_user_id,iam_user_id,name,avatar_url,department_ids,job_title,job_level_id,job_family_id,employee_type,status,organization_synced_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
			u.ID, u.FeishuOpenID, u.FeishuUserID, u.IAMUserID, u.Name, u.AvatarURL, u.DepartmentIDs, u.JobTitle, u.JobLevelID, u.JobFamilyID, u.EmployeeType, u.Status, u.OrganizationSyncedAt)
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO role_assignments(user_id,role) VALUES($1,$2)`, u.ID, domain.RoleEmployee)
		}
	} else {
		var conflicting bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM users
			WHERE id<>$1 AND (iam_user_id=$2 OR (feishu_user_id IS NOT NULL AND feishu_user_id=$3))
		)`, u.ID, *u.IAMUserID, u.FeishuUserID).Scan(&conflicting); err != nil {
			return domain.User{}, err
		}
		if conflicting {
			return domain.User{}, ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE users SET
			iam_user_id=$2,feishu_user_id=$3,name=$4,
			avatar_url=CASE WHEN $5<>'' THEN $5 ELSE avatar_url END,
			department_ids=$6,job_title=$7,job_level_id=$8,job_family_id=$9,
			employee_type=$10,status=$11,organization_synced_at=$12,updated_at=now()
			WHERE id=$1`,
			u.ID, u.IAMUserID, u.FeishuUserID, u.Name, u.AvatarURL, u.DepartmentIDs, u.JobTitle, u.JobLevelID, u.JobFamilyID, u.EmployeeType, u.Status, u.OrganizationSyncedAt)
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, ErrConflict
		}
		return domain.User{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return p.GetUser(ctx, u.ID)
}

func (p *Postgres) GetUser(ctx context.Context, id string) (domain.User, error) {
	return p.user(ctx, `WHERE u.id=$1`, id)
}
func (p *Postgres) GetUserByOpenID(ctx context.Context, id string) (domain.User, error) {
	return p.user(ctx, `WHERE u.feishu_open_id=$1`, id)
}
func (p *Postgres) GetUserByIAMID(ctx context.Context, iamUserID int64) (domain.User, error) {
	return p.user(ctx, `WHERE u.iam_user_id=$1`, iamUserID)
}
func (p *Postgres) user(ctx context.Context, clause string, arg any) (domain.User, error) {
	var u domain.User
	var roles []string
	err := p.pool.QueryRow(ctx, `SELECT u.id,u.feishu_open_id,COALESCE(u.feishu_user_id,''),u.iam_user_id,u.name,u.avatar_url,u.department_ids,u.job_title,u.job_level_id,u.job_family_id,u.employee_type,u.status,u.organization_synced_at,ARRAY(SELECT role FROM role_assignments WHERE user_id=u.id) FROM users u `+clause, arg).Scan(&u.ID, &u.FeishuOpenID, &u.FeishuUserID, &u.IAMUserID, &u.Name, &u.AvatarURL, &u.DepartmentIDs, &u.JobTitle, &u.JobLevelID, &u.JobFamilyID, &u.EmployeeType, &u.Status, &u.OrganizationSyncedAt, &roles)
	if err == pgx.ErrNoRows {
		return u, ErrNotFound
	}
	u.Roles = toRoles(roles)
	return u, err
}
func (p *Postgres) ListUsers(ctx context.Context) ([]domain.User, error) {
	rows, err := p.pool.Query(ctx, `SELECT u.id,u.feishu_open_id,COALESCE(u.feishu_user_id,''),u.iam_user_id,u.name,u.avatar_url,u.department_ids,u.job_title,u.job_level_id,u.job_family_id,u.employee_type,u.status,u.organization_synced_at,ARRAY(SELECT role FROM role_assignments WHERE user_id=u.id) FROM users u ORDER BY u.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.User{}
	for rows.Next() {
		var u domain.User
		var roles []string
		if err = rows.Scan(&u.ID, &u.FeishuOpenID, &u.FeishuUserID, &u.IAMUserID, &u.Name, &u.AvatarURL, &u.DepartmentIDs, &u.JobTitle, &u.JobLevelID, &u.JobFamilyID, &u.EmployeeType, &u.Status, &u.OrganizationSyncedAt, &roles); err != nil {
			return nil, err
		}
		u.Roles = toRoles(roles)
		out = append(out, u)
	}
	return out, rows.Err()
}

func (p *Postgres) UpdateUserRoles(ctx context.Context, userID string, roles []domain.Role) (domain.User, error) {
	normalized, err := domain.NormalizeRoles(roles)
	if err != nil {
		return domain.User{}, err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback(ctx)
	var exists, ownedSuper bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1), EXISTS(SELECT 1 FROM role_assignments WHERE user_id=$1 AND role='super_admin')`, userID).Scan(&exists, &ownedSuper); err != nil {
		return domain.User{}, err
	}
	if !exists {
		return domain.User{}, ErrNotFound
	}
	wantsSuper := false
	for _, role := range normalized {
		wantsSuper = wantsSuper || role == domain.RoleSuperAdmin
	}
	if ownedSuper && !wantsSuper {
		var count int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM role_assignments WHERE role='super_admin'`).Scan(&count); err != nil {
			return domain.User{}, err
		}
		if count <= 1 {
			return domain.User{}, ErrConflict
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM role_assignments WHERE user_id=$1`, userID); err != nil {
		return domain.User{}, err
	}
	for _, role := range normalized {
		if _, err = tx.Exec(ctx, `INSERT INTO role_assignments(user_id,role) VALUES($1,$2)`, userID, role); err != nil {
			return domain.User{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return p.GetUser(ctx, userID)
}

func (p *Postgres) UpdateUserStatus(ctx context.Context, openID, status string) error {
	tag, err := p.pool.Exec(ctx, `UPDATE users SET status=$2,updated_at=now() WHERE feishu_open_id=$1`, openID, status)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) CreateConversation(ctx context.Context, v domain.Conversation) error {
	normalizeConversation(&v)
	_, err := p.pool.Exec(ctx, `INSERT INTO conversations(id,user_id,title,agent_key,channel,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.ID, v.UserID, v.Title, v.AgentKey, v.Channel, v.CreatedAt, v.UpdatedAt)
	return err
}
func (p *Postgres) ListConversations(ctx context.Context, userID string) ([]domain.Conversation, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,user_id,title,agent_key,channel,created_at,updated_at FROM conversations WHERE user_id=$1 AND deleted_at IS NULL ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Conversation{}
	for rows.Next() {
		var v domain.Conversation
		if err = rows.Scan(&v.ID, &v.UserID, &v.Title, &v.AgentKey, &v.Channel, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p *Postgres) GetConversation(ctx context.Context, id string) (domain.Conversation, error) {
	var v domain.Conversation
	err := p.pool.QueryRow(ctx, `SELECT id,user_id,title,agent_key,channel,created_at,updated_at FROM conversations WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&v.ID, &v.UserID, &v.Title, &v.AgentKey, &v.Channel, &v.CreatedAt, &v.UpdatedAt)
	if err == pgx.ErrNoRows {
		return v, ErrNotFound
	}
	return v, err
}
func (p *Postgres) DeleteConversation(ctx context.Context, id, userID string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE conversations SET deleted_at=now() WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`, id, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM conversation_contexts WHERE conversation_id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM conversation_bindings WHERE conversation_id=$1`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) AddMessage(ctx context.Context, v domain.Message) error {
	citations := mustJSON(v.Citations)
	var reminderActionID any
	var meetingActionID any
	if v.ReminderAction != nil && v.ReminderAction.ID != "" {
		reminderActionID = v.ReminderAction.ID
	}
	if v.MeetingBookingAction != nil && v.MeetingBookingAction.ID != "" {
		meetingActionID = v.MeetingBookingAction.ID
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO messages(id,conversation_id,role,content,citations,model,prompt_tokens,completion_tokens,intent,standalone_query,context_version,reminder_action_id,meeting_booking_action_id,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, v.ID, v.ConversationID, v.Role, v.Content, citations, v.Model, v.PromptTokens, v.CompletionTokens, v.Intent, v.StandaloneQuery, v.ContextVersion, reminderActionID, meetingActionID, v.CreatedAt)
	if err != nil {
		return err
	}
	title := truncate(v.Content, 24)
	_, err = tx.Exec(ctx, `UPDATE conversations SET updated_at=$2,title=CASE WHEN title='新会话' AND $3='user' THEN $4 ELSE title END WHERE id=$1`, v.ConversationID, v.CreatedAt, v.Role, title)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) ListMessages(ctx context.Context, id string) ([]domain.Message, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,conversation_id,role,content,citations,model,intent,standalone_query,context_version,prompt_tokens,completion_tokens,COALESCE(reminder_action_id::text,''),COALESCE(meeting_booking_action_id::text,''),created_at FROM messages WHERE conversation_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Message{}
	actionIDs := []string{}
	meetingActionIDs := []string{}
	for rows.Next() {
		var v domain.Message
		var raw []byte
		var actionID string
		var meetingActionID string
		if err = rows.Scan(&v.ID, &v.ConversationID, &v.Role, &v.Content, &raw, &v.Model, &v.Intent, &v.StandaloneQuery, &v.ContextVersion, &v.PromptTokens, &v.CompletionTokens, &actionID, &meetingActionID, &v.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &v.Citations)
		out = append(out, v)
		actionIDs = append(actionIDs, actionID)
		meetingActionIDs = append(meetingActionIDs, meetingActionID)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i, actionID := range actionIDs {
		if actionID == "" {
			continue
		}
		if action, actionErr := p.GetReminderAction(ctx, actionID); actionErr == nil {
			out[i].ReminderAction = &action
		}
	}
	for i, actionID := range meetingActionIDs {
		if actionID == "" {
			continue
		}
		if action, actionErr := p.GetMeetingBookingAction(ctx, actionID); actionErr == nil {
			out[i].MeetingBookingAction = &action
		}
	}
	return out, nil
}

func (p *Postgres) CreateSource(ctx context.Context, v domain.KnowledgeSource) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO knowledge_sources(id,name,type,remote_token,default_acl,sync_status,sync_error,last_sync_stats,last_synced_at,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, v.ID, v.Name, v.Type, v.RemoteToken, mustJSON(v.DefaultACL), v.SyncStatus, v.SyncError, mustJSON(v.LastSyncStats), v.LastSyncedAt, v.CreatedAt)
	return err
}
func (p *Postgres) GetSource(ctx context.Context, id string) (domain.KnowledgeSource, error) {
	var v domain.KnowledgeSource
	var aclRaw, statsRaw []byte
	err := p.pool.QueryRow(ctx, `SELECT id,name,type,remote_token,default_acl,sync_status,sync_error,last_sync_stats,last_synced_at,created_at FROM knowledge_sources WHERE id=$1`, id).Scan(&v.ID, &v.Name, &v.Type, &v.RemoteToken, &aclRaw, &v.SyncStatus, &v.SyncError, &statsRaw, &v.LastSyncedAt, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.KnowledgeSource{}, ErrNotFound
	}
	if err != nil {
		return domain.KnowledgeSource{}, err
	}
	_ = json.Unmarshal(aclRaw, &v.DefaultACL)
	_ = json.Unmarshal(statsRaw, &v.LastSyncStats)
	return v, nil
}
func (p *Postgres) ListSources(ctx context.Context) ([]domain.KnowledgeSource, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,name,type,remote_token,default_acl,sync_status,sync_error,last_sync_stats,last_synced_at,created_at FROM knowledge_sources ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.KnowledgeSource{}
	for rows.Next() {
		var v domain.KnowledgeSource
		var aclRaw, statsRaw []byte
		if err = rows.Scan(&v.ID, &v.Name, &v.Type, &v.RemoteToken, &aclRaw, &v.SyncStatus, &v.SyncError, &statsRaw, &v.LastSyncedAt, &v.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(aclRaw, &v.DefaultACL)
		_ = json.Unmarshal(statsRaw, &v.LastSyncStats)
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p *Postgres) UpdateSource(ctx context.Context, v domain.KnowledgeSource) error {
	tag, err := p.pool.Exec(ctx, `UPDATE knowledge_sources SET name=$2,remote_token=$3,default_acl=$4,sync_status=$5,sync_error=$6,last_sync_stats=$7,last_synced_at=$8 WHERE id=$1`, v.ID, v.Name, v.RemoteToken, mustJSON(v.DefaultACL), v.SyncStatus, v.SyncError, mustJSON(v.LastSyncStats), v.LastSyncedAt)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
func (p *Postgres) CreateDocument(ctx context.Context, d domain.Document, chunks []domain.Chunk) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var source any
	if d.SourceID != "" {
		source = d.SourceID
	}
	_, err = tx.Exec(ctx, `INSERT INTO documents(id,source_id,title,source_url,remote_token,acl,status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, d.ID, source, d.Title, d.SourceURL, d.RemoteToken, mustJSON(d.ACL), d.Status, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return err
	}
	for _, v := range d.Versions {
		_, err = tx.Exec(ctx, `INSERT INTO document_versions(id,document_id,version,checksum,object_key,mime_type,status,effective_at,expires_at,published_at,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, v.ID, d.ID, v.Version, v.Checksum, v.ObjectKey, v.MimeType, v.Status, v.EffectiveAt, v.ExpiresAt, v.PublishedAt, v.CreatedAt)
		if err != nil {
			return err
		}
	}
	for _, c := range chunks {
		_, err = tx.Exec(ctx, `INSERT INTO chunks(id,document_id,version_id,ordinal,heading,page,content,embedding) VALUES($1,$2,$3,$4,$5,$6,$7,$8::vector)`, c.ID, c.DocumentID, c.VersionID, c.Ordinal, c.Heading, c.Page, c.Content, vectorArg(c.Embedding))
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (p *Postgres) CreateDocumentVersion(ctx context.Context, d domain.Document, v domain.DocumentVersion, chunks []domain.Chunk) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE documents SET title=$2,source_url=$3,remote_token=$4,acl=$5,status=$6,updated_at=$7 WHERE id=$1`, d.ID, d.Title, d.SourceURL, d.RemoteToken, mustJSON(d.ACL), d.Status, d.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_, err = tx.Exec(ctx, `INSERT INTO document_versions(id,document_id,version,checksum,object_key,mime_type,status,effective_at,expires_at,published_at,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, v.ID, d.ID, v.Version, v.Checksum, v.ObjectKey, v.MimeType, v.Status, v.EffectiveAt, v.ExpiresAt, v.PublishedAt, v.CreatedAt)
	if err != nil {
		return err
	}
	for _, c := range chunks {
		_, err = tx.Exec(ctx, `INSERT INTO chunks(id,document_id,version_id,ordinal,heading,page,content,embedding) VALUES($1,$2,$3,$4,$5,$6,$7,$8::vector)`, c.ID, c.DocumentID, c.VersionID, c.Ordinal, c.Heading, c.Page, c.Content, vectorArg(c.Embedding))
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (p *Postgres) UpdateDocument(ctx context.Context, d domain.Document) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE documents SET title=$2,source_url=$3,remote_token=$4,acl=$5,status=$6,updated_at=$7 WHERE id=$1`, d.ID, d.Title, d.SourceURL, d.RemoteToken, mustJSON(d.ACL), d.Status, d.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	for _, v := range d.Versions {
		_, err = tx.Exec(ctx, `UPDATE document_versions SET status=$2,effective_at=$3,expires_at=$4,published_at=$5 WHERE id=$1`, v.ID, v.Status, v.EffectiveAt, v.ExpiresAt, v.PublishedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (p *Postgres) GetDocument(ctx context.Context, id string) (domain.Document, error) {
	docs, err := p.documents(ctx, `WHERE d.id=$1`, id)
	if err != nil {
		return domain.Document{}, err
	}
	if len(docs) == 0 {
		return domain.Document{}, ErrNotFound
	}
	return docs[0], nil
}
func (p *Postgres) GetDocumentByRemoteToken(ctx context.Context, sourceID, remoteToken string) (domain.Document, error) {
	docs, err := p.documents(ctx, `WHERE d.source_id=$1 AND d.remote_token=$2`, sourceID, remoteToken)
	if err != nil {
		return domain.Document{}, err
	}
	if len(docs) == 0 {
		return domain.Document{}, ErrNotFound
	}
	return docs[0], nil
}
func (p *Postgres) ListDocuments(ctx context.Context) ([]domain.Document, error) {
	return p.documents(ctx, "ORDER BY d.updated_at DESC")
}
func (p *Postgres) ListDocumentsBySource(ctx context.Context, sourceID string) ([]domain.Document, error) {
	return p.documents(ctx, `WHERE d.source_id=$1 ORDER BY d.updated_at DESC`, sourceID)
}
func (p *Postgres) documents(ctx context.Context, clause string, args ...any) ([]domain.Document, error) {
	rows, err := p.pool.Query(ctx, `SELECT d.id,COALESCE(d.source_id::text,''),d.title,d.source_url,d.remote_token,d.acl,d.status,d.created_at,d.updated_at FROM documents d `+clause, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Document{}
	for rows.Next() {
		var d domain.Document
		var raw []byte
		if err = rows.Scan(&d.ID, &d.SourceID, &d.Title, &d.SourceURL, &d.RemoteToken, &raw, &d.Status, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &d.ACL)
		versions, versionErr := p.versions(ctx, d.ID)
		if versionErr != nil {
			return nil, versionErr
		}
		d.Versions = versions
		out = append(out, d)
	}
	return out, rows.Err()
}
func (p *Postgres) versions(ctx context.Context, docID string) ([]domain.DocumentVersion, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,version,checksum,object_key,mime_type,status,effective_at,expires_at,published_at,created_at FROM document_versions WHERE document_id=$1 ORDER BY created_at DESC`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.DocumentVersion{}
	for rows.Next() {
		var v domain.DocumentVersion
		if err = rows.Scan(&v.ID, &v.Version, &v.Checksum, &v.ObjectKey, &v.MimeType, &v.Status, &v.EffectiveAt, &v.ExpiresAt, &v.PublishedAt, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p *Postgres) SearchChunks(ctx context.Context, user domain.User, query string, embedding []float32, limit int) ([]domain.Chunk, map[string]domain.Document, error) {
	selectSQL := `SELECT c.id,c.document_id,c.version_id,c.ordinal,c.heading,c.page,c.content,d.title,d.source_url,d.acl,d.created_at,d.updated_at,v.version
		FROM chunks c
		JOIN documents d ON d.id=c.document_id
		JOIN document_versions v ON v.id=c.version_id
		WHERE d.status='published' AND v.status='published' AND (
			$8::boolean OR d.acl->>'scope'='all' OR
			EXISTS (
				SELECT 1 FROM jsonb_array_elements(COALESCE(d.acl->'rules','[]'::jsonb)) AS acl_rule
				WHERE
					(COALESCE(jsonb_array_length(acl_rule->'department_ids'),0)+COALESCE(jsonb_array_length(acl_rule->'job_titles'),0)+COALESCE(jsonb_array_length(acl_rule->'job_level_ids'),0)+COALESCE(jsonb_array_length(acl_rule->'job_family_ids'),0)+COALESCE(jsonb_array_length(acl_rule->'employee_types'),0)+COALESCE(jsonb_array_length(acl_rule->'user_ids'),0)) > 0 AND
					(COALESCE(jsonb_array_length(acl_rule->'department_ids'),0)=0 OR EXISTS (SELECT 1 FROM jsonb_array_elements_text(acl_rule->'department_ids') x WHERE x=ANY($2::text[]))) AND
					(COALESCE(jsonb_array_length(acl_rule->'job_titles'),0)=0 OR EXISTS (SELECT 1 FROM jsonb_array_elements_text(acl_rule->'job_titles') x WHERE x=$3)) AND
					(COALESCE(jsonb_array_length(acl_rule->'job_level_ids'),0)=0 OR EXISTS (SELECT 1 FROM jsonb_array_elements_text(acl_rule->'job_level_ids') x WHERE x=$4)) AND
					(COALESCE(jsonb_array_length(acl_rule->'job_family_ids'),0)=0 OR EXISTS (SELECT 1 FROM jsonb_array_elements_text(acl_rule->'job_family_ids') x WHERE x=$5)) AND
					(COALESCE(jsonb_array_length(acl_rule->'employee_types'),0)=0 OR EXISTS (SELECT 1 FROM jsonb_array_elements_text(acl_rule->'employee_types') x WHERE x=$6)) AND
					(COALESCE(jsonb_array_length(acl_rule->'user_ids'),0)=0 OR EXISTS (SELECT 1 FROM jsonb_array_elements_text(acl_rule->'user_ids') x WHERE x=$1))
			) OR
			EXISTS (SELECT 1 FROM jsonb_array_elements_text(COALESCE(d.acl->'user_ids','[]'::jsonb)) x WHERE x=$1) OR
			EXISTS (SELECT 1 FROM jsonb_array_elements_text(COALESCE(d.acl->'department_ids','[]'::jsonb)) x WHERE x=ANY($2::text[])) OR
			EXISTS (SELECT 1 FROM jsonb_array_elements_text(COALESCE(d.acl->'role_names','[]'::jsonb)) x WHERE x=ANY($7::text[]))
		)`
	searchQuery := strings.Join(tokenize(query), " ")
	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = string(role)
	}
	employeeType := fmt.Sprint(user.EmployeeType)
	aclArgs := []any{user.ID, user.DepartmentIDs, user.JobTitle, user.JobLevelID, user.JobFamilyID, employeeType, roleNames, user.HasRole(domain.RoleSuperAdmin)}
	var rows pgx.Rows
	var err error
	if len(embedding) > 0 {
		rows, err = p.pool.Query(ctx, selectSQL+` ORDER BY (CASE WHEN c.embedding IS NULL THEN 0 ELSE 1-(c.embedding <=> $9::vector) END)*0.7 + ts_rank(c.search_vector,plainto_tsquery('simple',$10))*0.3 DESC LIMIT 200`, append(aclArgs, vectorArg(embedding), searchQuery)...)
	} else {
		rows, err = p.pool.Query(ctx, selectSQL+` ORDER BY ts_rank(c.search_vector,plainto_tsquery('simple',$9)) DESC LIMIT 200`, append(aclArgs, searchQuery)...)
	}
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	type scored struct {
		c     domain.Chunk
		score int
	}
	items := []scored{}
	docs := map[string]domain.Document{}
	terms := tokenize(query)
	for rows.Next() {
		var c domain.Chunk
		var title, url, version string
		var raw []byte
		var created, updated time.Time
		if err = rows.Scan(&c.ID, &c.DocumentID, &c.VersionID, &c.Ordinal, &c.Heading, &c.Page, &c.Content, &title, &url, &raw, &created, &updated, &version); err != nil {
			return nil, nil, err
		}
		var acl domain.ACL
		_ = json.Unmarshal(raw, &acl)
		if !acl.Allows(user) {
			continue
		}
		lower := strings.ToLower(c.Content + " " + c.Heading + " " + title)
		score := 0
		for _, term := range terms {
			score += strings.Count(lower, term)
		}
		if score > 0 {
			items = append(items, scored{c, score})
			docs[c.DocumentID] = domain.Document{ID: c.DocumentID, Title: title, SourceURL: url, ACL: acl, Status: "published", Versions: []domain.DocumentVersion{{ID: c.VersionID, Version: version, Status: "published"}}, CreatedAt: created, UpdatedAt: updated}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].score > items[j].score })
	if limit <= 0 {
		limit = 8
	}
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]domain.Chunk, len(items))
	for i := range items {
		out[i] = items[i].c
	}
	return out, docs, rows.Err()
}

func (p *Postgres) ListConfigs(ctx context.Context) ([]domain.AgentConfigVersion, error) {
	rows, err := p.pool.Query(ctx, `SELECT id,version,status,config,created_by,created_at,published_at FROM agent_config_versions ORDER BY version DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AgentConfigVersion{}
	for rows.Next() {
		var v domain.AgentConfigVersion
		var raw []byte
		if err = rows.Scan(&v.ID, &v.Version, &v.Status, &raw, &v.CreatedBy, &v.CreatedAt, &v.PublishedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &v.Config)
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p *Postgres) SaveConfig(ctx context.Context, v domain.AgentConfigVersion) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO agent_config_versions(id,version,status,config,created_by,created_at,published_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.ID, v.Version, v.Status, mustJSON(v.Config), v.CreatedBy, v.CreatedAt, v.PublishedAt)
	return err
}
func (p *Postgres) PublishConfig(ctx context.Context, id string) (domain.AgentConfigVersion, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.AgentConfigVersion{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE agent_config_versions SET status='archived' WHERE status='published'`)
	if err != nil {
		return domain.AgentConfigVersion{}, err
	}
	tag, err := tx.Exec(ctx, `UPDATE agent_config_versions SET status='published',published_at=now() WHERE id=$1`, id)
	if err != nil {
		return domain.AgentConfigVersion{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.AgentConfigVersion{}, ErrNotFound
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.AgentConfigVersion{}, err
	}
	configs, err := p.ListConfigs(ctx)
	for _, v := range configs {
		if v.ID == id {
			return v, nil
		}
	}
	return domain.AgentConfigVersion{}, err
}
func (p *Postgres) PublishedConfig(ctx context.Context) (domain.AgentConfigVersion, error) {
	var v domain.AgentConfigVersion
	var raw []byte
	err := p.pool.QueryRow(ctx, `SELECT id,version,status,config,created_by,created_at,published_at FROM agent_config_versions WHERE status='published' ORDER BY version DESC LIMIT 1`).Scan(&v.ID, &v.Version, &v.Status, &raw, &v.CreatedBy, &v.CreatedAt, &v.PublishedAt)
	if err == pgx.ErrNoRows {
		return v, ErrNotFound
	}
	_ = json.Unmarshal(raw, &v.Config)
	return v, err
}

const agentProfileSelect = `SELECT id,agent_key,name,description,kind,provider,model,enabled,credential_source,encrypted_api_key,api_key_hint,settings,COALESCE(created_by::text,''),COALESCE(updated_by::text,''),created_at,updated_at FROM agent_profiles `

func (p *Postgres) ListAgentProfiles(ctx context.Context) ([]domain.AgentProfile, error) {
	rows, err := p.pool.Query(ctx, agentProfileSelect+`ORDER BY created_at,agent_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []domain.AgentProfile{}
	for rows.Next() {
		value, scanErr := scanAgentProfile(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (p *Postgres) GetAgentProfile(ctx context.Context, id string) (domain.AgentProfile, error) {
	value, err := scanAgentProfile(p.pool.QueryRow(ctx, agentProfileSelect+`WHERE id=$1`, id))
	if err == pgx.ErrNoRows {
		return value, ErrNotFound
	}
	return value, err
}

func (p *Postgres) GetAgentProfileByKey(ctx context.Context, key string) (domain.AgentProfile, error) {
	value, err := scanAgentProfile(p.pool.QueryRow(ctx, agentProfileSelect+`WHERE agent_key=$1`, key))
	if err == pgx.ErrNoRows {
		return value, ErrNotFound
	}
	return value, err
}

func (p *Postgres) UpsertAgentProfile(ctx context.Context, value domain.AgentProfile) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO agent_profiles(id,agent_key,name,description,kind,provider,model,enabled,credential_source,encrypted_api_key,api_key_hint,settings,created_by,updated_by,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		ON CONFLICT(id) DO UPDATE SET agent_key=EXCLUDED.agent_key,name=EXCLUDED.name,description=EXCLUDED.description,kind=EXCLUDED.kind,provider=EXCLUDED.provider,model=EXCLUDED.model,enabled=EXCLUDED.enabled,credential_source=EXCLUDED.credential_source,encrypted_api_key=EXCLUDED.encrypted_api_key,api_key_hint=EXCLUDED.api_key_hint,settings=EXCLUDED.settings,updated_by=EXCLUDED.updated_by,updated_at=EXCLUDED.updated_at`,
		value.ID, value.AgentKey, value.Name, value.Description, value.Kind, value.Provider, value.Model, value.Enabled, value.CredentialSource, value.EncryptedAPIKey, value.APIKeyHint, mustJSON(value.Settings), nullUUID(value.CreatedBy), nullUUID(value.UpdatedBy), value.CreatedAt, value.UpdatedAt)
	return err
}

func scanAgentProfile(row notificationRow) (domain.AgentProfile, error) {
	var value domain.AgentProfile
	var settings []byte
	err := row.Scan(&value.ID, &value.AgentKey, &value.Name, &value.Description, &value.Kind, &value.Provider, &value.Model, &value.Enabled, &value.CredentialSource, &value.EncryptedAPIKey, &value.APIKeyHint, &settings, &value.CreatedBy, &value.UpdatedBy, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return value, err
	}
	_ = json.Unmarshal(settings, &value.Settings)
	if value.Settings == nil {
		value.Settings = map[string]any{}
	}
	return value, nil
}

func (p *Postgres) CreateNotification(ctx context.Context, v domain.NotificationDraft) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO notification_drafts(id,title,content,content_format,images,recipient_type,recipients,audience,status,scheduled_at,approved_by,created_by,idempotency_key,last_error,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULL,$11,$12,$13,$14,$15)`,
		v.ID, v.Title, v.Content, v.ContentFormat, mustJSON(v.Images), v.RecipientType, mustJSON(v.Recipients), mustJSON(v.Audience), v.Status, v.ScheduledAt, v.CreatedBy, v.IdempotencyKey, v.LastError, v.CreatedAt, v.UpdatedAt)
	return err
}
func (p *Postgres) UpdateNotification(ctx context.Context, v domain.NotificationDraft) error {
	var approved any
	if v.ApprovedBy != "" {
		approved = v.ApprovedBy
	}
	tag, err := p.pool.Exec(ctx, `UPDATE notification_drafts SET title=$2,content=$3,content_format=$4,images=$5,recipient_type=$6,recipients=$7,audience=$8,status=$9,scheduled_at=$10,approved_by=$11,last_error=$12,updated_at=$13 WHERE id=$1`,
		v.ID, v.Title, v.Content, v.ContentFormat, mustJSON(v.Images), v.RecipientType, mustJSON(v.Recipients), mustJSON(v.Audience), v.Status, v.ScheduledAt, approved, v.LastError, v.UpdatedAt)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
func (p *Postgres) GetNotification(ctx context.Context, id string) (domain.NotificationDraft, error) {
	values, err := p.notifications(ctx, `WHERE n.id=$1`, id)
	if err != nil {
		return domain.NotificationDraft{}, err
	}
	if len(values) == 0 {
		return domain.NotificationDraft{}, ErrNotFound
	}
	return values[0], nil
}
func (p *Postgres) ListNotifications(ctx context.Context) ([]domain.NotificationDraft, error) {
	return p.notifications(ctx, `ORDER BY n.updated_at DESC`)
}
func (p *Postgres) notifications(ctx context.Context, clause string, args ...any) ([]domain.NotificationDraft, error) {
	rows, err := p.pool.Query(ctx, notificationSelect+clause, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users, _ := p.ListUsers(ctx)
	out := []domain.NotificationDraft{}
	for rows.Next() {
		v, scanErr := scanNotification(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		if len(v.Recipients) > 0 {
			v.RecipientCount = len(v.Recipients)
		} else {
			for _, u := range users {
				if v.Audience.Allows(u) {
					v.RecipientCount++
				}
			}
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

const notificationSelect = `SELECT n.id,n.title,n.content,n.content_format,n.images,n.recipient_type,n.recipients,n.audience,n.status,n.scheduled_at,COALESCE(n.approved_by::text,''),n.created_by,n.idempotency_key,n.last_error,n.created_at,n.updated_at FROM notification_drafts n `

type notificationRow interface {
	Scan(...any) error
}

func scanNotification(row notificationRow) (domain.NotificationDraft, error) {
	var value domain.NotificationDraft
	var images, recipients, audience []byte
	err := row.Scan(&value.ID, &value.Title, &value.Content, &value.ContentFormat, &images, &value.RecipientType, &recipients, &audience, &value.Status, &value.ScheduledAt, &value.ApprovedBy, &value.CreatedBy, &value.IdempotencyKey, &value.LastError, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return value, err
	}
	_ = json.Unmarshal(images, &value.Images)
	_ = json.Unmarshal(recipients, &value.Recipients)
	_ = json.Unmarshal(audience, &value.Audience)
	value.RecipientCount = len(value.Recipients)
	return value, nil
}

func (p *Postgres) ClaimDueNotifications(ctx context.Context, now time.Time, limit int) ([]domain.NotificationDraft, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := p.pool.Query(ctx, `WITH due AS (
		SELECT id FROM notification_drafts
		WHERE (status='scheduled' AND scheduled_at IS NOT NULL AND scheduled_at<=$1)
			OR (status='sending' AND updated_at<=$1-interval '10 minutes')
		ORDER BY COALESCE(scheduled_at,updated_at) FOR UPDATE SKIP LOCKED LIMIT $2
	) UPDATE notification_drafts n SET status='sending',updated_at=$1
	FROM due WHERE n.id=due.id
	RETURNING n.id,n.title,n.content,n.content_format,n.images,n.recipient_type,n.recipients,n.audience,n.status,n.scheduled_at,COALESCE(n.approved_by::text,''),n.created_by,n.idempotency_key,n.last_error,n.created_at,n.updated_at`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]domain.NotificationDraft, 0, limit)
	for rows.Next() {
		value, scanErr := scanNotification(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (p *Postgres) RecordNotificationDelivery(ctx context.Context, notificationID, receiverID, receiverType, receiverName, status, messageID, errorMessage string, attempts int, updatedAt time.Time) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO delivery_attempts(notification_id,receiver_open_id,receiver_type,receiver_name,status,message_id,error,attempts,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT(notification_id,receiver_open_id) DO UPDATE SET receiver_type=EXCLUDED.receiver_type,receiver_name=EXCLUDED.receiver_name,status=EXCLUDED.status,message_id=EXCLUDED.message_id,error=EXCLUDED.error,attempts=delivery_attempts.attempts+1,updated_at=EXCLUDED.updated_at`,
		notificationID, receiverID, receiverType, receiverName, status, messageID, errorMessage, attempts, updatedAt)
	return err
}

func (p *Postgres) AppendAudit(ctx context.Context, v domain.AuditEvent) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO audit_events(id,actor_id,action,resource_type,resource_id,metadata,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, v.ID, nullUUID(v.ActorID), v.Action, v.ResourceType, v.ResourceID, mustJSON(v.Metadata), v.CreatedAt)
	return err
}
func (p *Postgres) ListAudit(ctx context.Context, limit int) ([]domain.AuditEvent, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := p.pool.Query(ctx, `SELECT a.id,COALESCE(a.actor_id::text,''),COALESCE(u.name,'系统'),a.action,a.resource_type,a.resource_id,a.metadata,a.created_at FROM audit_events a LEFT JOIN users u ON u.id=a.actor_id ORDER BY a.created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AuditEvent{}
	for rows.Next() {
		var v domain.AuditEvent
		var raw []byte
		if err = rows.Scan(&v.ID, &v.ActorID, &v.ActorName, &v.Action, &v.ResourceType, &v.ResourceID, &raw, &v.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &v.Metadata)
		out = append(out, v)
	}
	return out, rows.Err()
}
func (p *Postgres) MarkEventProcessed(ctx context.Context, id string) bool {
	tag, err := p.pool.Exec(ctx, `INSERT INTO processed_events(event_id) VALUES($1) ON CONFLICT DO NOTHING`, id)
	return err == nil && tag.RowsAffected() == 1
}
func (p *Postgres) ForgetProcessedEvent(ctx context.Context, id string) {
	_, _ = p.pool.Exec(ctx, `DELETE FROM processed_events WHERE event_id=$1`, id)
}

func (p *Postgres) RecordMessageFeedback(ctx context.Context, messageID, userID string, positive bool) error {
	tag, err := p.pool.Exec(ctx, `INSERT INTO message_feedback(message_id,user_id,positive,created_at,updated_at)
		SELECT m.id,c.user_id,$3,now(),now()
		FROM messages m JOIN conversations c ON c.id=m.conversation_id
		WHERE m.id=$1 AND c.user_id=$2 AND m.role='assistant'
		ON CONFLICT(message_id) DO UPDATE SET positive=EXCLUDED.positive,user_id=EXCLUDED.user_id,updated_at=now()`,
		messageID, userID, positive)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) Metrics(ctx context.Context) domain.DashboardMetrics {
	var m domain.DashboardMetrics
	_ = p.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM messages WHERE role='user' AND created_at>=date_trunc('day',now())),
			COALESCE((SELECT AVG(CASE WHEN positive THEN 1.0 ELSE 0.0 END)::float8 FROM message_feedback),0),
			COALESCE((SELECT AVG(CASE WHEN jsonb_array_length(citations)=0 THEN 1.0 ELSE 0.0 END)::float8 FROM messages WHERE role='assistant'),0),
			COALESCE((SELECT AVG(CASE WHEN jsonb_array_length(citations)>0 THEN 1.0 ELSE 0.0 END)::float8 FROM messages WHERE role='assistant'),0),
			(SELECT COUNT(*) FROM documents WHERE status='published'),
			(SELECT COUNT(*) FROM knowledge_sources WHERE sync_status IN ('queued','pending','syncing')),
			COALESCE((SELECT AVG(CASE WHEN status='sent' THEN 1.0 ELSE 0.0 END)::float8 FROM delivery_attempts WHERE status IN ('sent','failed')),0)
	`).Scan(&m.QuestionsToday, &m.PositiveRate, &m.NoAnswerRate, &m.CitationCoverage, &m.DocumentsPublished, &m.SyncBacklog, &m.DeliverySuccess)
	return m
}

func mustJSON(v any) []byte { data, _ := json.Marshal(v); return data }
func toRoles(values []string) []domain.Role {
	out := make([]domain.Role, len(values))
	for i, v := range values {
		out[i] = domain.Role(v)
	}
	return out
}
func nullUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func vectorArg(values []float32) any {
	if len(values) == 0 {
		return nil
	}
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprintf("%.8f", value)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
func (p *Postgres) String() string { return fmt.Sprintf("postgres(%p)", p.pool) }
