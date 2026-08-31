package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"
	"time"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/integration/imageproxy"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/store"
)

var errGenerationStopped = context.Canceled

type recordingImageGenerator struct {
	method string
	model  string
}

func (g *recordingImageGenerator) record(method, model string) (imageproxy.Image, error) {
	g.method, g.model = method, model
	return imageproxy.Image{}, errGenerationStopped
}

func (g *recordingImageGenerator) GenerateChat(_ context.Context, model, _, _, _ string, _ []string) (imageproxy.Image, error) {
	return g.record("chat", model)
}
func (g *recordingImageGenerator) GenerateImage(_ context.Context, model, _, _, _ string) (imageproxy.Image, error) {
	return g.record("images", model)
}
func (g *recordingImageGenerator) GenerateGPTImage2(_ context.Context, model, _, _, _ string, _ []imageproxy.ReferenceImage) (imageproxy.Image, error) {
	return g.record("gpt_image_2", model)
}
func (g *recordingImageGenerator) GenerateUniversalEdit(_ context.Context, model, _, _, _ string, _ []imageproxy.ReferenceImage) (imageproxy.Image, error) {
	return g.record("universal_edit", model)
}

type fixedImageGenerator struct {
	image imageproxy.Image
	err   error
}

func (g fixedImageGenerator) GenerateChat(context.Context, string, string, string, string, []string) (imageproxy.Image, error) {
	return g.image, g.err
}

func (g fixedImageGenerator) GenerateImage(context.Context, string, string, string, string) (imageproxy.Image, error) {
	return g.image, g.err
}

func (g fixedImageGenerator) GenerateGPTImage2(context.Context, string, string, string, string, []imageproxy.ReferenceImage) (imageproxy.Image, error) {
	return g.image, g.err
}

func (g fixedImageGenerator) GenerateUniversalEdit(context.Context, string, string, string, string, []imageproxy.ReferenceImage) (imageproxy.Image, error) {
	return g.image, g.err
}

func TestImageModelResolutionCapabilities(t *testing.T) {
	tests := []struct {
		model string
		want  []string
	}{
		{model: "gemini-3.1-flash-image", want: []string{"1K", "2K", "4K"}},
		{model: "google/gemini-3-pro-image-preview", want: []string{"1K", "2K", "4K"}},
		{model: "gemini-2.5-flash-image", want: []string{"1K"}},
		{model: "gemini-3.1-flash-lite-image", want: []string{"1K"}},
		{model: "unknown-image-model", want: nil},
	}
	for _, test := range tests {
		got := knownImageModelSizes(test.model)
		if !equalStrings(got, test.want) {
			t.Fatalf("%s: got %#v, want %#v", test.model, got, test.want)
		}
	}
	if got := defaultImageModelSizes("unknown-image-model"); !equalStrings(got, []string{"1K"}) {
		t.Fatalf("unknown synced models must default to 1K, got %#v", got)
	}
	if got := effectiveImageModelSizes("gemini-2.5-flash-image", []string{"1K", "2K", "4K"}); !equalStrings(got, []string{"1K"}) {
		t.Fatalf("known 1K-only model exposed unsupported sizes: %#v", got)
	}
	if got := effectiveImageModelSizes("gemini-2.5-flash-image", []string{"2K", "4K"}); !equalStrings(got, []string{"1K"}) {
		t.Fatalf("an existing invalid capability set must fall back to 1K: %#v", got)
	}
}

func TestTransientImageErrorsRetryAtMostThreeAttempts(t *testing.T) {
	if !shouldRetryImageError(context.DeadlineExceeded, 2) || shouldRetryImageError(context.DeadlineExceeded, 3) {
		t.Fatal("transient errors should retry at most three attempts")
	}
}

func TestGenerateOnePersistsImageBelowRequestedResolution(t *testing.T) {
	data := testPNG(t, 1024, 1024)
	repo := store.NewMemory(domain.AgentConfig{})
	blobs := newImageTestBlob()
	agent, err := NewImageAgent(repo, repo, blobs, security.NoopScanner{}, "test-image-secret")
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := NewImageDispatcher(agent, 0)
	err = dispatcher.generateOne(context.Background(), domain.ImageJob{
		ID: "job-resolution", UserID: "user", ProjectID: "project", Prompt: "cat",
		AspectRatio: "1:1", ImageSize: "2K", Attempts: 1,
	}, domain.ImageModel{ModelID: "gemini-3.1-flash-image", Protocol: "chat_completions"},
		"comfly", fixedImageGenerator{image: imageproxy.Image{Data: data, MIME: "image/png"}}, nil, nil,
		domain.ImageJobOutput{ID: "output-resolution", JobID: "job-resolution"})
	if err != nil {
		t.Fatalf("a valid image should be persisted regardless of requested resolution: %v", err)
	}
	if len(blobs.objects) != 1 {
		t.Fatalf("generated image was not persisted: %#v", blobs.objects)
	}
}

func TestGPTImage2ResolutionWarningKeepsDowngradedResult(t *testing.T) {
	if warning := gptImageResolutionWarning("gpt-image-2", "1:1", "2K", 1254, 1254); warning == "" {
		t.Fatal("a 2K request returning the native 1K result must be reported")
	}
	if warning := gptImageResolutionWarning("gpt-image-2", "1:1", "2K", 2048, 2048); warning != "" {
		t.Fatalf("a real 2K result must not be warned: %s", warning)
	}
	if warning := gptImageResolutionWarning("gpt-image-2", "1:1", "1K", 1254, 1254); warning != "" {
		t.Fatalf("1254 square is a valid GPT Image 2 1K result: %s", warning)
	}
}

func TestGPTImage2ModelDetectionIsExact(t *testing.T) {
	if !isGPTImage2Model("gpt-image-2") || !isGPTImage2Model("openai/gpt-image-2") {
		t.Fatal("GPT Image 2 aliases must be detected")
	}
	if isGPTImage2Model("gpt-image-2-all") {
		t.Fatal("the web-backed gpt-image-2-all model must not use the official Images protocol")
	}
}

func TestSyncedModelsChooseRelayCompatibleRequests(t *testing.T) {
	now := time.Now()
	comfly := syncedImageModels(domain.ImageRelay{ID: "comfly", RelayKey: "comfly"}, []imageproxy.RemoteModel{{
		ID: "gpt-image-2", SupportedEndpointTypes: []string{"openai"},
	}}, now)
	if len(comfly) != 1 || comfly[0].Protocol != "gpt_image_2" || !comfly[0].SupportsReference || comfly[0].RequestModelID != "gpt-image-2" {
		t.Fatalf("Comfly GPT Image 2 must use the concrete-size Images contract: %+v", comfly)
	}

	xgapi := syncedImageModels(domain.ImageRelay{ID: "xgapi", RelayKey: "xgapi"}, []imageproxy.RemoteModel{
		{ID: "gemini-3.1-flash-image-preview", SupportedEndpointTypes: []string{"openai", "image-generation"}},
		{ID: "gemini-3.1-flash-image-preview-2k", SupportedEndpointTypes: []string{"openai", "gemini"}},
	}, now)
	if len(xgapi) != 2 || xgapi[1].RequestModelID != "gemini-3.1-flash-image-preview" || !xgapi[1].SupportsReference {
		t.Fatalf("XGAPI resolution alias must call the base image-generation model: %+v", xgapi)
	}
}

func TestGenerateOneRoutesComflyGPTToImagesProtocol(t *testing.T) {
	dispatcher := &ImageDispatcher{}
	generator := &recordingImageGenerator{}
	model := domain.ImageModel{ModelID: "gpt-image-2", RequestModelID: "gpt-image-2", Protocol: "gpt_image_2"}
	_ = dispatcher.generateOne(context.Background(), domain.ImageJob{Prompt: "poster", ImageSize: "2K", AspectRatio: "1:1"},
		model, "comfly", generator, []string{"data:image/png;base64,eA=="}, []imageproxy.ReferenceImage{{Data: []byte("image"), Width: 1024, Height: 1024}}, domain.ImageJobOutput{})
	if generator.method != "gpt_image_2" || generator.model != "gpt-image-2" {
		t.Fatalf("unexpected Comfly route: method=%s model=%s", generator.method, generator.model)
	}
}

func TestGenerateOneRoutesXGAPIGPTBeforeUniversalEdit(t *testing.T) {
	dispatcher := &ImageDispatcher{}
	generator := &recordingImageGenerator{}
	model := domain.ImageModel{ModelID: "gpt-image-2", RequestModelID: "gpt-image-2", Protocol: "gpt_image_2"}
	_ = dispatcher.generateOne(context.Background(), domain.ImageJob{Prompt: "poster", ImageSize: "2K", AspectRatio: "1:1"},
		model, "xgapi", generator, nil, []imageproxy.ReferenceImage{{Data: []byte("image"), Width: 1024, Height: 1024}}, domain.ImageJobOutput{})
	if generator.method != "gpt_image_2" {
		t.Fatalf("XGAPI GPT Image 2 must not use the Gemini universal edit route: %s", generator.method)
	}
}

func TestGenerateOneRoutesXGAPIAliasToBaseEditModel(t *testing.T) {
	dispatcher := &ImageDispatcher{}
	generator := &recordingImageGenerator{}
	model := domain.ImageModel{ModelID: "gemini-3.1-flash-image-preview-2k", RequestModelID: "gemini-3.1-flash-image-preview", Protocol: "chat_completions"}
	_ = dispatcher.generateOne(context.Background(), domain.ImageJob{Prompt: "poster", ImageSize: "2K", AspectRatio: "1:1"},
		model, "xgapi", generator, nil, []imageproxy.ReferenceImage{{Data: []byte("image")}}, domain.ImageJobOutput{})
	if generator.method != "universal_edit" || generator.model != "gemini-3.1-flash-image-preview" {
		t.Fatalf("unexpected XGAPI route: method=%s model=%s", generator.method, generator.model)
	}
}

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := png.Encode(&output, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
