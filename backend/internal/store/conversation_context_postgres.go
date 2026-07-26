package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
)

func (p *Postgres) UpdateMessageAnalysis(ctx context.Context, messageID, intent, standalone string, version, promptTokens, completionTokens int) error {
	tag, err := p.pool.Exec(ctx, `UPDATE messages SET intent=$2,standalone_query=$3,context_version=$4,
		prompt_tokens=CASE WHEN $5>=0 THEN $5 ELSE prompt_tokens END,
		completion_tokens=CASE WHEN $6>=0 THEN $6 ELSE completion_tokens END
		WHERE id=$1`, messageID, intent, standalone, version, promptTokens, completionTokens)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (p *Postgres) GetConversationContext(ctx context.Context, conversationID string) (domain.ConversationContext, error) {
	var value domain.ConversationContext
	var activeTask []byte
	err := p.pool.QueryRow(ctx, `SELECT conversation_id,summary,COALESCE(summarized_through_message_id::text,''),
		COALESCE(reset_through_message_id::text,''),active_task,version,expires_at,updated_at
		FROM conversation_contexts WHERE conversation_id=$1`, conversationID).
		Scan(&value.ConversationID, &value.Summary, &value.SummarizedThroughMessageID, &value.ResetThroughMessageID, &activeTask, &value.Version, &value.ExpiresAt, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	if err == nil {
		_ = json.Unmarshal(activeTask, &value.ActiveTask)
	}
	return value, err
}

func (p *Postgres) SaveConversationContext(ctx context.Context, value domain.ConversationContext) error {
	var summarizedThrough any
	if value.SummarizedThroughMessageID != "" {
		summarizedThrough = value.SummarizedThroughMessageID
	}
	var resetThrough any
	if value.ResetThroughMessageID != "" {
		resetThrough = value.ResetThroughMessageID
	}
	_, err := p.pool.Exec(ctx, `INSERT INTO conversation_contexts(conversation_id,summary,summarized_through_message_id,reset_through_message_id,active_task,version,expires_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT(conversation_id) DO UPDATE SET summary=EXCLUDED.summary,
			summarized_through_message_id=EXCLUDED.summarized_through_message_id,
			reset_through_message_id=EXCLUDED.reset_through_message_id,active_task=EXCLUDED.active_task,
			version=EXCLUDED.version,expires_at=EXCLUDED.expires_at,updated_at=EXCLUDED.updated_at`,
		value.ConversationID, value.Summary, summarizedThrough, resetThrough, mustJSON(value.ActiveTask), value.Version, value.ExpiresAt, value.UpdatedAt)
	return err
}

func (p *Postgres) ResetConversationContext(ctx context.Context, conversationID string) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO conversation_contexts(
			conversation_id,summary,summarized_through_message_id,reset_through_message_id,active_task,version,expires_at,updated_at)
		SELECT $1,'',NULL,(SELECT id FROM messages WHERE conversation_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1),'{}',0,NULL,now()
		WHERE EXISTS(SELECT 1 FROM conversations WHERE id=$1 AND deleted_at IS NULL)
		ON CONFLICT(conversation_id) DO UPDATE SET summary='',summarized_through_message_id=NULL,
			reset_through_message_id=EXCLUDED.reset_through_message_id,active_task='{}',version=conversation_contexts.version+1,
			expires_at=NULL,updated_at=now()`, conversationID)
	return err
}

func (p *Postgres) GetOrCreateBoundConversation(ctx context.Context, user domain.User, agentKey, channel, scopeID string, now time.Time, ttl time.Duration) (domain.Conversation, error) {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return domain.Conversation{}, err
	}
	defer tx.Rollback(ctx)
	lockKey := user.ID + "\x00" + agentKey + "\x00" + channel + "\x00" + scopeID
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); err != nil {
		return domain.Conversation{}, err
	}
	var conversationID string
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `SELECT conversation_id,expires_at FROM conversation_bindings
		WHERE user_id=$1 AND agent_key=$2 AND channel=$3 AND external_scope_id=$4 FOR UPDATE`,
		user.ID, agentKey, channel, scopeID).Scan(&conversationID, &expiresAt)
	if err == nil && expiresAt.After(now) {
		var conversation domain.Conversation
		err = tx.QueryRow(ctx, `SELECT id,user_id,title,agent_key,channel,created_at,updated_at
			FROM conversations WHERE id=$1 AND deleted_at IS NULL`, conversationID).
			Scan(&conversation.ID, &conversation.UserID, &conversation.Title, &conversation.AgentKey, &conversation.Channel, &conversation.CreatedAt, &conversation.UpdatedAt)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE conversation_bindings SET expires_at=$2,updated_at=$3 WHERE conversation_id=$1`, conversationID, now.Add(ttl), now)
			if err == nil {
				err = tx.Commit(ctx)
			}
			return conversation, err
		}
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return domain.Conversation{}, err
	}
	conversation := domain.Conversation{
		ID: ids.New("conv"), UserID: user.ID, Title: "飞书行政助手",
		AgentKey: agentKey, Channel: channel, CreatedAt: now, UpdatedAt: now,
	}
	normalizeConversation(&conversation)
	_, err = tx.Exec(ctx, `INSERT INTO conversations(id,user_id,title,agent_key,channel,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, conversation.ID, conversation.UserID, conversation.Title, conversation.AgentKey, conversation.Channel, conversation.CreatedAt, conversation.UpdatedAt)
	if err != nil {
		return domain.Conversation{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO conversation_bindings(user_id,agent_key,channel,external_scope_id,conversation_id,expires_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT(user_id,agent_key,channel,external_scope_id) DO UPDATE SET
			conversation_id=EXCLUDED.conversation_id,expires_at=EXCLUDED.expires_at,updated_at=EXCLUDED.updated_at`,
		user.ID, agentKey, channel, scopeID, conversation.ID, now.Add(ttl), now)
	if err == nil {
		err = tx.Commit(ctx)
	}
	return conversation, err
}

func (p *Postgres) ResetConversationBinding(ctx context.Context, userID, agentKey, channel, scopeID string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM conversation_bindings WHERE user_id=$1 AND agent_key=$2 AND channel=$3 AND external_scope_id=$4`, userID, agentKey, channel, scopeID)
	return err
}

func (p *Postgres) CleanupConversationHistory(ctx context.Context, before time.Time) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE messages SET content='[内容已按留存策略清理]',citations='[]',standalone_query=''
		WHERE created_at<$1 AND content<>'[内容已按留存策略清理]'`, before); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM conversation_contexts WHERE updated_at<$1`, before); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM conversation_bindings WHERE updated_at<$1`, before); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
