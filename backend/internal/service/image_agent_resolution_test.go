package service

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"testing"

	"internal-ai-agent/backend/internal/domain"
	"internal-ai-agent/backend/internal/integration/imageproxy"
	"internal-ai-agent/backend/internal/security"
	"internal-ai-agent/backend/internal/store"
)

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

func TestImageResolutionAcceptedAllowsTenPercentTolerance(t *testing.T) {
	if !imageResolutionAccepted(1844, 1844, 2048, 2048) {
		t.Fatal("dimensions within the ten percent tolerance should pass")
	}
	if imageResolutionAccepted(1843, 1843, 2048, 2048) {
		t.Fatal("dimensions below the ten percent tolerance should fail")
	}
	if imageResolutionAccepted(1344, 768, 2688, 1536) {
		t.Fatal("a 1K landscape must not satisfy a 2K request")
	}
	if imageResolutionAccepted(768, 1344, 3072, 5376) {
		t.Fatal("a 1K portrait must not satisfy a 4K request")
	}
}

func TestLowResolutionRetriesExactlyOnce(t *testing.T) {
	err := &imageResolutionError{RequestedTier: "4K", AspectRatio: "1:1", ActualWidth: 1024, ActualHeight: 1024}
	if !shouldRetryImageError(err, 1) {
		t.Fatal("the first low-resolution response should be retried")
	}
	if shouldRetryImageError(err, 2) {
		t.Fatal("a low-resolution response must not be retried more than once")
	}
	if !shouldRetryImageError(context.DeadlineExceeded, 2) || shouldRetryImageError(context.DeadlineExceeded, 3) {
		t.Fatal("existing transient-error retry limit should remain three attempts")
	}
}

func TestGenerateOneDoesNotPersistLowResolutionImage(t *testing.T) {
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
		fixedImageGenerator{image: imageproxy.Image{Data: data, MIME: "image/png"}}, nil,
		domain.ImageJobOutput{ID: "output-resolution", JobID: "job-resolution"})
	var resolutionErr *imageResolutionError
	if !errors.As(err, &resolutionErr) {
		t.Fatalf("expected imageResolutionError, got %v", err)
	}
	if len(blobs.objects) != 0 {
		t.Fatalf("low-resolution image must not be persisted: %#v", blobs.objects)
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
