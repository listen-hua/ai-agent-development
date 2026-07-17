package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"internal-ai-agent/backend/internal/blob"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/ids"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/model"
	"internal-ai-agent/backend/internal/parser"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/store"
)

type DriveReader interface {
	Configured() bool
	ListDriveFolder(context.Context, string) ([]feishu.DriveItem, error)
	FetchDriveItem(context.Context, feishu.DriveItem) (feishu.DriveFile, error)
	GetWikiNode(context.Context, string) (feishu.WikiNode, error)
	ListWikiNodes(context.Context, string, string) ([]feishu.WikiNode, error)
}

type Knowledge struct {
	repo           store.Repository
	provider       model.Provider
	parser         *parser.Tika
	embeddingModel string
	blob           blob.Store
	scanner        security.Scanner
	drive          DriveReader
	syncMu         sync.Mutex
	syncing        map[string]bool
}

func NewKnowledge(repo store.Repository, provider model.Provider, parser *parser.Tika, embeddingModel string, blobStore blob.Store, scanner security.Scanner, drive DriveReader) *Knowledge {
	return &Knowledge{repo: repo, provider: provider, parser: parser, embeddingModel: embeddingModel, blob: blobStore, scanner: scanner, drive: drive, syncing: map[string]bool{}}
}

func (k *Knowledge) CreateSource(ctx context.Context, actor domain.User, name, sourceType, token string, acl domain.ACL) (domain.KnowledgeSource, error) {
	if err := acl.Validate(); err != nil {
		return domain.KnowledgeSource{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.KnowledgeSource{}, errors.New("source name is required")
	}
	sourceType = inferConnectedSourceType(sourceType, token)
	var err error
	switch sourceType {
	case "feishu_folder":
		token, err = feishu.ExtractFolderToken(token)
	case "feishu_wiki":
		token, err = feishu.ExtractWikiNodeToken(token)
	default:
		return domain.KnowledgeSource{}, errors.New("connected source type must be feishu_folder or feishu_wiki")
	}
	if err != nil {
		return domain.KnowledgeSource{}, err
	}
	now := time.Now()
	value := domain.KnowledgeSource{ID: ids.New("src"), Name: name, Type: sourceType, RemoteToken: token, DefaultACL: acl, SyncStatus: "idle", CreatedAt: now}
	err = k.repo.CreateSource(ctx, value)
	if err == nil {
		k.audit(ctx, actor, "knowledge.source.create", "knowledge_source", value.ID)
	}
	return value, err
}

func inferConnectedSourceType(sourceType, token string) string {
	normalized := strings.ToLower(strings.TrimSpace(token))
	if strings.Contains(normalized, "/wiki/") || (!strings.Contains(normalized, "://") && strings.HasPrefix(normalized, "wik")) {
		return "feishu_wiki"
	}
	if strings.Contains(normalized, "/drive/folder/") || (!strings.Contains(normalized, "://") && strings.HasPrefix(normalized, "fld")) {
		return "feishu_folder"
	}
	return sourceType
}

func (k *Knowledge) Ingest(ctx context.Context, actor domain.User, title, mime string, data []byte, acl domain.ACL) (domain.Document, error) {
	if err := acl.Validate(); err != nil {
		return domain.Document{}, err
	}
	if err := k.scanner.Scan(ctx, data); err != nil {
		return domain.Document{}, fmt.Errorf("file security scan failed: %w", err)
	}
	content, err := k.parser.Extract(ctx, data, mime)
	if err != nil {
		return domain.Document{}, err
	}
	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])
	now := time.Now()
	docID := ids.New("doc")
	versionID := ids.New("ver")
	objectKey := "documents/" + docID + "/" + versionID + "/original"
	if err = k.blob.Put(ctx, objectKey, data, mime); err != nil {
		return domain.Document{}, fmt.Errorf("store original file: %w", err)
	}
	version := domain.DocumentVersion{ID: versionID, Version: "1.0", Checksum: checksum, ObjectKey: objectKey, MimeType: mime, Status: "draft", CreatedAt: now}
	doc := domain.Document{ID: docID, Title: title, ACL: acl, Status: "draft", Versions: []domain.DocumentVersion{version}, CreatedAt: now, UpdatedAt: now}
	chunks := splitChunks(docID, versionID, content, 900, 120)
	texts := make([]string, len(chunks))
	for i := range chunks {
		texts[i] = chunks[i].Content
	}
	if k.provider.Available() && len(texts) > 0 {
		if vectors, embedErr := k.provider.Embed(ctx, k.embeddingModel, texts, 1024); embedErr == nil {
			for i := range chunks {
				if i < len(vectors) {
					chunks[i].Embedding = vectors[i]
				}
			}
		}
	}
	if err = k.repo.CreateDocument(ctx, doc, chunks); err == nil {
		k.audit(ctx, actor, "knowledge.document.upload", "document", doc.ID)
	}
	return doc, err
}

func (k *Knowledge) Publish(ctx context.Context, actor domain.User, id string) (domain.Document, error) {
	doc, err := k.repo.GetDocument(ctx, id)
	if err != nil {
		return doc, err
	}
	if doc.Status == "unavailable" {
		return doc, errors.New("source document is unavailable; restore Feishu access before publishing")
	}
	latestDraft := -1
	for i := range doc.Versions {
		if doc.Versions[i].Status == "draft" {
			latestDraft = i
			break
		}
	}
	if latestDraft < 0 {
		return doc, errors.New("document has no draft version to publish")
	}
	now := time.Now()
	for i := range doc.Versions {
		if i == latestDraft {
			doc.Versions[i].Status = "published"
			doc.Versions[i].PublishedAt = &now
		} else if doc.Versions[i].Status == "published" || doc.Versions[i].Status == "draft" {
			doc.Versions[i].Status = "archived"
		}
	}
	doc.Status = "published"
	doc.UpdatedAt = now
	err = k.repo.UpdateDocument(ctx, doc)
	if err == nil {
		k.audit(ctx, actor, "knowledge.document.publish", "document", doc.ID)
	}
	return doc, err
}

func (k *Knowledge) UpdateACL(ctx context.Context, actor domain.User, id string, acl domain.ACL) (domain.Document, error) {
	if err := acl.Validate(); err != nil {
		return domain.Document{}, err
	}
	doc, err := k.repo.GetDocument(ctx, id)
	if err != nil {
		return domain.Document{}, err
	}
	doc.ACL = acl
	doc.UpdatedAt = time.Now()
	if err = k.repo.UpdateDocument(ctx, doc); err != nil {
		return domain.Document{}, err
	}
	k.audit(ctx, actor, "knowledge.document.acl_update", "document", doc.ID)
	return doc, nil
}

func (k *Knowledge) Sync(ctx context.Context, actor domain.User, id string) error {
	source, err := k.repo.GetSource(ctx, id)
	if err != nil {
		return err
	}
	if !isConnectedFeishuSource(source.Type) {
		return errors.New("this source cannot be synchronized")
	}
	if k.drive == nil || !k.drive.Configured() {
		return errors.New("FEISHU_APP_ID or FEISHU_APP_SECRET is not configured")
	}
	k.syncMu.Lock()
	if k.syncing[id] {
		k.syncMu.Unlock()
		return nil
	}
	k.syncing[id] = true
	k.syncMu.Unlock()

	source.SyncStatus = "queued"
	source.SyncError = ""
	if err = k.repo.UpdateSource(ctx, source); err != nil {
		k.finishSyncSlot(id)
		return err
	}
	k.audit(ctx, actor, "knowledge.source.sync_requested", "knowledge_source", id)
	go func() {
		defer k.finishSyncSlot(id)
		runCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if runErr := k.syncSource(runCtx, source); runErr != nil {
			slog.Warn("Feishu knowledge source sync failed", "source_id", source.ID, "error", runErr)
		}
	}()
	return nil
}

func (k *Knowledge) RunSourceScheduler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	k.queueAllSources(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			k.queueAllSources(ctx)
		}
	}
}

func (k *Knowledge) queueAllSources(ctx context.Context) {
	sources, err := k.repo.ListSources(ctx)
	if err != nil {
		slog.Warn("list scheduled knowledge sources failed", "error", err)
		return
	}
	for _, source := range sources {
		if !isConnectedFeishuSource(source.Type) {
			continue
		}
		if err = k.Sync(ctx, domain.User{Name: "system"}, source.ID); err != nil {
			slog.Warn("queue scheduled knowledge source sync failed", "source_id", source.ID, "error", err)
		}
	}
}

func (k *Knowledge) finishSyncSlot(id string) {
	k.syncMu.Lock()
	delete(k.syncing, id)
	k.syncMu.Unlock()
}

func (k *Knowledge) syncSource(ctx context.Context, source domain.KnowledgeSource) error {
	source.SyncStatus = "syncing"
	source.SyncError = ""
	source.LastSyncStats = domain.SyncStats{}
	if err := k.repo.UpdateSource(ctx, source); err != nil {
		return err
	}

	var items []feishu.DriveItem
	var err error
	switch source.Type {
	case "feishu_folder":
		items, err = k.walkDriveFolder(ctx, source.RemoteToken)
	case "feishu_wiki":
		items, err = k.walkWikiNodes(ctx, source.RemoteToken)
	default:
		err = errors.New("this source cannot be synchronized")
	}
	if err != nil {
		if feishu.IsAccessLoss(err) {
			source.LastSyncStats.Unavailable = k.markSourceUnavailable(ctx, source.ID)
		}
		source.SyncStatus = "error"
		source.SyncError = truncateError(err.Error(), 1000)
		_ = k.repo.UpdateSource(context.Background(), source)
		return err
	}
	source.LastSyncStats.Discovered = len(items)
	seen := make(map[string]bool, len(items))
	failures := make([]string, 0)

	for _, item := range items {
		resolved := resolvedDriveItem(item)
		if resolved.Token == "" || resolved.Type == "folder" {
			continue
		}
		if !supportedDriveItem(resolved) {
			source.LastSyncStats.Skipped++
			continue
		}
		identityToken := driveItemIdentity(resolved)
		seen[identityToken] = true
		outcome, itemErr := k.importDriveItem(ctx, source, resolved)
		if itemErr != nil {
			source.LastSyncStats.Failed++
			if feishu.IsAccessLoss(itemErr) {
				if document, getErr := k.repo.GetDocumentByRemoteToken(ctx, source.ID, identityToken); getErr == nil {
					document.Status = "unavailable"
					document.UpdatedAt = time.Now()
					if k.repo.UpdateDocument(ctx, document) == nil {
						source.LastSyncStats.Unavailable++
					}
				}
			}
			if len(failures) < 5 {
				failures = append(failures, resolved.Name+": "+itemErr.Error())
			}
			continue
		}
		switch outcome {
		case "created":
			source.LastSyncStats.Created++
		case "updated":
			source.LastSyncStats.Updated++
		default:
			source.LastSyncStats.Unchanged++
		}
	}

	existing, listErr := k.repo.ListDocumentsBySource(ctx, source.ID)
	if listErr != nil {
		failures = append(failures, listErr.Error())
		source.LastSyncStats.Failed++
	} else {
		for _, document := range existing {
			if document.RemoteToken == "" || seen[document.RemoteToken] || document.Status == "unavailable" {
				continue
			}
			document.Status = "unavailable"
			document.UpdatedAt = time.Now()
			if updateErr := k.repo.UpdateDocument(ctx, document); updateErr != nil {
				source.LastSyncStats.Failed++
				failures = append(failures, updateErr.Error())
				continue
			}
			source.LastSyncStats.Unavailable++
		}
	}

	now := time.Now()
	source.LastSyncedAt = &now
	if source.LastSyncStats.Failed > 0 {
		source.SyncStatus = "partial"
		source.SyncError = truncateError(strings.Join(failures, "; "), 1000)
	} else {
		source.SyncStatus = "success"
		source.SyncError = ""
	}
	if err = k.repo.UpdateSource(context.Background(), source); err != nil {
		return err
	}
	return nil
}

func (k *Knowledge) walkDriveFolder(ctx context.Context, rootToken string) ([]feishu.DriveItem, error) {
	queue := []string{rootToken}
	visited := map[string]bool{}
	items := make([]feishu.DriveItem, 0)
	for len(queue) > 0 {
		folderToken := queue[0]
		queue = queue[1:]
		if visited[folderToken] {
			continue
		}
		visited[folderToken] = true
		children, err := k.drive.ListDriveFolder(ctx, folderToken)
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			resolved := resolvedDriveItem(child)
			if resolved.Type == "folder" {
				queue = append(queue, resolved.Token)
				continue
			}
			items = append(items, child)
			if len(items) > 10000 {
				return nil, errors.New("Feishu source contains more than 10,000 documents")
			}
		}
	}
	return items, nil
}

func (k *Knowledge) walkWikiNodes(ctx context.Context, rootToken string) ([]feishu.DriveItem, error) {
	root, err := k.drive.GetWikiNode(ctx, rootToken)
	if err != nil {
		return nil, err
	}
	if root.SpaceID == "" {
		return nil, errors.New("飞书知识库节点缺少知识空间 ID")
	}
	items := []feishu.DriveItem{wikiNodeDriveItem(root)}
	queue := make([]feishu.WikiNode, 0)
	if root.HasChild {
		queue = append(queue, root)
	}
	visited := map[string]bool{}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		if parent.NodeToken == "" || visited[parent.NodeToken] {
			continue
		}
		visited[parent.NodeToken] = true
		children, listErr := k.drive.ListWikiNodes(ctx, parent.SpaceID, parent.NodeToken)
		if listErr != nil {
			return nil, listErr
		}
		for _, child := range children {
			if child.SpaceID == "" {
				child.SpaceID = parent.SpaceID
			}
			items = append(items, wikiNodeDriveItem(child))
			if child.HasChild {
				queue = append(queue, child)
			}
			if len(items) > 10000 {
				return nil, errors.New("Feishu source contains more than 10,000 documents")
			}
		}
	}
	return items, nil
}

func wikiNodeDriveItem(node feishu.WikiNode) feishu.DriveItem {
	return feishu.DriveItem{
		Token:       node.ObjToken,
		RemoteToken: node.NodeToken,
		Name:        node.Title,
		Type:        node.ObjType,
	}
}

func (k *Knowledge) importDriveItem(ctx context.Context, source domain.KnowledgeSource, item feishu.DriveItem) (string, error) {
	identityToken := driveItemIdentity(item)
	file, err := k.drive.FetchDriveItem(ctx, item)
	if err != nil {
		return "", err
	}
	if len(file.Data) == 0 {
		return "", errors.New("document is empty")
	}
	if err = k.scanner.Scan(ctx, file.Data); err != nil {
		return "", fmt.Errorf("file security scan failed: %w", err)
	}
	content, err := k.extractContent(ctx, file.Data, file.MimeType)
	if err != nil {
		return "", fmt.Errorf("parse document: %w", err)
	}
	if strings.TrimSpace(content) == "" {
		return "", errors.New("no readable text was found")
	}
	sum := sha256.Sum256(file.Data)
	checksum := hex.EncodeToString(sum[:])
	existing, getErr := k.repo.GetDocumentByRemoteToken(ctx, source.ID, identityToken)
	if getErr != nil && !errors.Is(getErr, store.ErrNotFound) {
		return "", getErr
	}
	if getErr == nil && len(existing.Versions) > 0 && existing.Versions[0].Checksum == checksum {
		existing.Title = item.Name
		existing.SourceURL = item.URL
		existing.Status = liveDocumentStatus(existing.Versions)
		existing.UpdatedAt = time.Now()
		if err = k.repo.UpdateDocument(ctx, existing); err != nil {
			return "", err
		}
		return "unchanged", nil
	}

	now := time.Now()
	documentID := existing.ID
	if documentID == "" {
		documentID = ids.New("doc")
	}
	versionID := ids.New("ver")
	objectKey := "documents/" + documentID + "/" + versionID + "/original"
	if err = k.blob.Put(ctx, objectKey, file.Data, file.MimeType); err != nil {
		return "", fmt.Errorf("store original file: %w", err)
	}
	version := domain.DocumentVersion{
		ID: versionID, Version: fmt.Sprintf("%d.0", len(existing.Versions)+1), Checksum: checksum,
		ObjectKey: objectKey, MimeType: file.MimeType, Status: "draft", CreatedAt: now,
	}
	document := domain.Document{
		ID: documentID, SourceID: source.ID, Title: item.Name, SourceURL: item.URL, RemoteToken: identityToken,
		ACL: source.DefaultACL, Status: "draft", Versions: []domain.DocumentVersion{version}, CreatedAt: now, UpdatedAt: now,
	}
	if getErr == nil {
		document.ACL = existing.ACL
		document.CreatedAt = existing.CreatedAt
		document.Status = liveDocumentStatus(existing.Versions)
	}
	chunks := splitChunks(document.ID, version.ID, content, 900, 120)
	k.embedChunks(ctx, chunks)
	if getErr == nil {
		if err = k.repo.CreateDocumentVersion(ctx, document, version, chunks); err != nil {
			return "", err
		}
		return "updated", nil
	}
	if err = k.repo.CreateDocument(ctx, document, chunks); err != nil {
		return "", err
	}
	return "created", nil
}

func (k *Knowledge) extractContent(ctx context.Context, data []byte, mime string) (string, error) {
	if strings.HasPrefix(strings.ToLower(mime), "text/plain") || strings.HasPrefix(strings.ToLower(mime), "text/markdown") {
		return string(data), nil
	}
	return k.parser.Extract(ctx, data, mime)
}

func (k *Knowledge) embedChunks(ctx context.Context, chunks []domain.Chunk) {
	if !k.provider.Available() || len(chunks) == 0 {
		return
	}
	texts := make([]string, len(chunks))
	for i := range chunks {
		texts[i] = chunks[i].Content
	}
	vectors, err := k.provider.Embed(ctx, k.embeddingModel, texts, 1024)
	if err != nil {
		slog.Warn("embedding Feishu document failed; keyword retrieval remains available", "error", err)
		return
	}
	for i := range chunks {
		if i < len(vectors) {
			chunks[i].Embedding = vectors[i]
		}
	}
}

func (k *Knowledge) markSourceUnavailable(ctx context.Context, sourceID string) int {
	documents, err := k.repo.ListDocumentsBySource(ctx, sourceID)
	if err != nil {
		return 0
	}
	count := 0
	for _, document := range documents {
		if document.Status == "unavailable" {
			continue
		}
		document.Status = "unavailable"
		document.UpdatedAt = time.Now()
		if k.repo.UpdateDocument(ctx, document) == nil {
			count++
		}
	}
	return count
}

func resolvedDriveItem(item feishu.DriveItem) feishu.DriveItem {
	if item.Type == "shortcut" && item.ShortcutInfo.TargetToken != "" {
		item.Token = item.ShortcutInfo.TargetToken
		item.Type = item.ShortcutInfo.TargetType
	}
	return item
}

func driveItemIdentity(item feishu.DriveItem) string {
	if item.RemoteToken != "" {
		return item.RemoteToken
	}
	return item.Token
}

func isConnectedFeishuSource(sourceType string) bool {
	return sourceType == "feishu_folder" || sourceType == "feishu_wiki"
}

func supportedDriveItem(item feishu.DriveItem) bool {
	switch item.Type {
	case "docx", "doc", "sheet", "bitable":
		return true
	case "file":
		extension := strings.ToLower(filepath.Ext(item.Name))
		switch extension {
		case ".pdf", ".docx", ".pptx", ".xlsx", ".html", ".htm", ".txt", ".md":
			return true
		}
	}
	return false
}

func liveDocumentStatus(versions []domain.DocumentVersion) string {
	for _, version := range versions {
		if version.Status == "published" {
			return "published"
		}
	}
	return "draft"
}

func truncateError(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= limit {
		return value
	}
	return string([]rune(value)[:limit])
}
func (k *Knowledge) audit(ctx context.Context, actor domain.User, action, resource, id string) {
	_ = k.repo.AppendAudit(ctx, domain.AuditEvent{ID: ids.New("aud"), ActorID: actor.ID, ActorName: actor.Name, Action: action, ResourceType: resource, ResourceID: id, Metadata: map[string]any{}, CreatedAt: time.Now()})
}

func splitChunks(documentID, versionID, content string, size, overlap int) []domain.Chunk {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	runes := []rune(content)
	out := []domain.Chunk{}
	for start, ordinal := 0, 0; start < len(runes); ordinal++ {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		segment := strings.TrimSpace(string(runes[start:end]))
		if utf8.RuneCountInString(segment) > 0 {
			out = append(out, domain.Chunk{ID: ids.New("chk"), DocumentID: documentID, VersionID: versionID, Ordinal: ordinal, Content: segment})
		}
		if end == len(runes) {
			break
		}
		start = end - overlap
		if start < 0 {
			start = 0
		}
	}
	return out
}
func checksumLabel(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return fmt.Sprint(value)
}
