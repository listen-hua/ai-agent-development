package service

import (
	"context"
	"testing"

	"internal-ai-agent/backend/internal/blob"
	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/integration/feishu"
	"internal-ai-agent/backend/internal/model"
	"internal-ai-agent/backend/internal/parser"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/store"
)

func TestCreateSourceInfersWikiFromLinkWhenClientSendsFolderDefault(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	knowledge := NewKnowledge(repo, model.Mock{}, parser.New(""), "text-embedding-v3", blob.Noop{}, security.NoopScanner{}, nil)

	source, err := knowledge.CreateSource(
		ctx,
		domain.User{Name: "admin"},
		"行政知识库",
		"feishu_folder",
		"https://example.feishu.cn/wiki/wikcnRoot?from=from_copylink",
		domain.ACL{Scope: "all"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if source.Type != "feishu_wiki" || source.RemoteToken != "wikcnRoot" {
		t.Fatalf("expected inferred Wiki source, got %#v", source)
	}
}

type fakeDriveReader struct {
	items        []feishu.DriveItem
	files        map[string]feishu.DriveFile
	wikiRoot     feishu.WikiNode
	wikiChildren map[string][]feishu.WikiNode
}

func (*fakeDriveReader) Configured() bool { return true }
func (f *fakeDriveReader) ListDriveFolder(context.Context, string) ([]feishu.DriveItem, error) {
	return append([]feishu.DriveItem(nil), f.items...), nil
}
func (f *fakeDriveReader) FetchDriveItem(_ context.Context, item feishu.DriveItem) (feishu.DriveFile, error) {
	return f.files[item.Token], nil
}
func (f *fakeDriveReader) GetWikiNode(context.Context, string) (feishu.WikiNode, error) {
	return f.wikiRoot, nil
}
func (f *fakeDriveReader) ListWikiNodes(_ context.Context, _ string, parentToken string) ([]feishu.WikiNode, error) {
	return append([]feishu.WikiNode(nil), f.wikiChildren[parentToken]...), nil
}

func TestFeishuSourceSyncCreatesVersionsAndMarksMissingDocuments(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	drive := &fakeDriveReader{
		items: []feishu.DriveItem{{Token: "doccn_policy", Name: "请假制度", Type: "docx", URL: "https://example.feishu.cn/docx/doccn_policy"}},
		files: map[string]feishu.DriveFile{"doccn_policy": {Data: []byte("第一版：年假需提前申请。"), MimeType: "text/plain; charset=utf-8", Name: "请假制度.txt"}},
	}
	knowledge := NewKnowledge(repo, model.Mock{}, parser.New(""), "text-embedding-v3", blob.Noop{}, security.NoopScanner{}, drive)
	source := domain.KnowledgeSource{ID: "src-1", Name: "行政制度", Type: "feishu_folder", RemoteToken: "fld-root", DefaultACL: domain.ACL{Scope: "all"}, SyncStatus: "idle"}
	if err := repo.CreateSource(ctx, source); err != nil {
		t.Fatal(err)
	}

	if err := knowledge.syncSource(ctx, source); err != nil {
		t.Fatal(err)
	}
	documents, _ := repo.ListDocumentsBySource(ctx, source.ID)
	if len(documents) != 1 || documents[0].Status != "draft" || len(documents[0].Versions) != 1 {
		t.Fatalf("unexpected first import: %#v", documents)
	}
	document, err := knowledge.Publish(ctx, domain.User{Name: "admin"}, documents[0].ID)
	if err != nil || document.Status != "published" {
		t.Fatalf("publish failed: %#v, %v", document, err)
	}

	drive.files["doccn_policy"] = feishu.DriveFile{Data: []byte("第二版：年假需至少提前三个工作日申请。"), MimeType: "text/plain; charset=utf-8", Name: "请假制度.txt"}
	if err = knowledge.syncSource(ctx, source); err != nil {
		t.Fatal(err)
	}
	documents, _ = repo.ListDocumentsBySource(ctx, source.ID)
	if len(documents[0].Versions) != 2 || documents[0].Versions[0].Status != "draft" || documents[0].Status != "published" {
		t.Fatalf("new source content should create a draft without replacing the published version: %#v", documents[0])
	}
	drive.files["doccn_policy"] = feishu.DriveFile{Data: []byte("第三版：年假需至少提前五个工作日申请。"), MimeType: "text/plain; charset=utf-8", Name: "请假制度.txt"}
	if err = knowledge.syncSource(ctx, source); err != nil {
		t.Fatal(err)
	}
	documents, _ = repo.ListDocumentsBySource(ctx, source.ID)
	document, err = knowledge.Publish(ctx, domain.User{Name: "admin"}, documents[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	published := 0
	for _, version := range document.Versions {
		if version.Status == "published" {
			published++
		}
	}
	if published != 1 || document.Versions[0].Status != "published" {
		t.Fatalf("only the newest draft should be published: %#v", document.Versions)
	}

	drive.items = nil
	if err = knowledge.syncSource(ctx, source); err != nil {
		t.Fatal(err)
	}
	documents, _ = repo.ListDocumentsBySource(ctx, source.ID)
	if documents[0].Status != "unavailable" {
		t.Fatalf("missing source document should be unavailable, got %q", documents[0].Status)
	}
	updatedSource, _ := repo.GetSource(ctx, source.ID)
	if updatedSource.SyncStatus != "success" || updatedSource.LastSyncStats.Unavailable != 1 {
		t.Fatalf("unexpected sync result: %#v", updatedSource)
	}
}

func TestFeishuWikiSourceSyncUsesNodeTokensAsDocumentIdentity(t *testing.T) {
	ctx := context.Background()
	repo := store.NewMemory(domain.AgentConfig{})
	drive := &fakeDriveReader{
		wikiRoot: feishu.WikiNode{
			SpaceID: "space-1", NodeToken: "wik-root", ObjToken: "docx-root",
			ObjType: "docx", Title: "行政制度", HasChild: true,
		},
		wikiChildren: map[string][]feishu.WikiNode{
			"wik-root": {{
				SpaceID: "space-1", NodeToken: "wik-leave", ObjToken: "docx-leave",
				ObjType: "docx", Title: "请假制度",
			}},
		},
		files: map[string]feishu.DriveFile{
			"docx-root":  {Data: []byte("行政制度总则"), MimeType: "text/plain; charset=utf-8", Name: "行政制度.txt"},
			"docx-leave": {Data: []byte("请假需提前申请"), MimeType: "text/plain; charset=utf-8", Name: "请假制度.txt"},
		},
	}
	knowledge := NewKnowledge(repo, model.Mock{}, parser.New(""), "text-embedding-v3", blob.Noop{}, security.NoopScanner{}, drive)
	source := domain.KnowledgeSource{ID: "src-wiki", Name: "行政知识库", Type: "feishu_wiki", RemoteToken: "wik-root", DefaultACL: domain.ACL{Scope: "all"}, SyncStatus: "idle"}
	if err := repo.CreateSource(ctx, source); err != nil {
		t.Fatal(err)
	}
	if err := knowledge.syncSource(ctx, source); err != nil {
		t.Fatal(err)
	}
	documents, err := repo.ListDocumentsBySource(ctx, source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 2 {
		t.Fatalf("expected root and child Wiki documents, got %#v", documents)
	}
	identities := map[string]bool{}
	for _, document := range documents {
		identities[document.RemoteToken] = true
	}
	if !identities["wik-root"] || !identities["wik-leave"] || identities["docx-root"] {
		t.Fatalf("Wiki node tokens must be used as stable identities: %#v", identities)
	}
}
