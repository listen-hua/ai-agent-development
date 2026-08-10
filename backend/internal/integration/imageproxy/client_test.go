package imageproxy

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestCollectImageValuesSupportsOpenAIShapes(t *testing.T) {
	pixel := base64.StdEncoding.EncodeToString([]byte("pixel"))
	response := map[string]any{
		"data": []any{map[string]any{"b64_json": pixel}},
		"choices": []any{map[string]any{"message": map[string]any{
			"images": []any{map[string]any{"image_url": map[string]any{"url": "https://cdn.example.com/a.png"}}},
		}}},
	}
	values := collectImageValues(response)
	if len(values) != 2 || values[0] != "data:image/png;base64,"+pixel {
		t.Fatalf("unexpected values: %#v", values)
	}
}

func TestPixelSizePreservesRatioAcrossTiers(t *testing.T) {
	if got := pixelSize("16:9", "2K"); got != "2688x1536" {
		t.Fatalf("unexpected size: %s", got)
	}
}

func TestChatGenerationInputUsesGeminiImageConfig(t *testing.T) {
	input := chatGenerationInput("gemini-3.1-flash-image", "draw a cat", "16:9", "4K", nil)
	extraBody, ok := input["extra_body"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected extra_body: %#v", input["extra_body"])
	}
	if _, exists := extraBody["aspect_ratio"]; exists {
		t.Fatal("aspect_ratio must be nested under google.image_config")
	}
	if _, exists := extraBody["image_size"]; exists {
		t.Fatal("image_size must be nested under google.image_config")
	}
	google, ok := extraBody["google"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected google config: %#v", extraBody["google"])
	}
	if _, exists := google["generation_config"]; exists {
		t.Fatal("obsolete google.generation_config must not be sent")
	}
	config, ok := google["image_config"].(map[string]string)
	if !ok {
		t.Fatalf("unexpected image_config: %#v", google["image_config"])
	}
	if config["aspect_ratio"] != "16:9" || config["image_size"] != "4K" {
		t.Fatalf("unexpected image_config values: %#v", config)
	}
}

func TestExpectedDimensionsAcrossRatiosAndTiers(t *testing.T) {
	tests := []struct {
		ratio, tier   string
		width, height int
	}{
		{ratio: "1:1", tier: "1K", width: 1024, height: 1024},
		{ratio: "16:9", tier: "2K", width: 2688, height: 1536},
		{ratio: "9:16", tier: "4K", width: 3072, height: 5376},
		{ratio: "4:3", tier: "4K", width: 4608, height: 3584},
	}
	for _, test := range tests {
		width, height := ExpectedDimensions(test.ratio, test.tier)
		if width != test.width || height != test.height {
			t.Fatalf("%s %s: got %dx%d, want %dx%d", test.ratio, test.tier, width, height, test.width, test.height)
		}
	}
}

func TestDecodeImageReportsRejectedHostWithoutFullURL(t *testing.T) {
	client, err := New("https://relay.example.com/v1", "test", time.Second, []string{"relay.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.decodeImage(context.Background(), "https://generated.example.net/private/result.png?token=secret")
	var hostErr *HostNotAllowedError
	if !errors.As(err, &hostErr) {
		t.Fatalf("expected HostNotAllowedError, got %v", err)
	}
	if hostErr.Host != "generated.example.net" {
		t.Fatalf("unexpected rejected host: %q", hostErr.Host)
	}
	if got := err.Error(); got != `relay image URL host "generated.example.net" is not allowed` {
		t.Fatalf("unexpected safe error: %q", got)
	}
}

func TestHostAllowedAcceptsConfiguredHostAndSubdomains(t *testing.T) {
	client, err := New("https://relay.example.com/v1", "test", time.Second, []string{"images.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if !client.hostAllowed("images.example.com") || !client.hostAllowed("cdn.images.example.com") {
		t.Fatal("configured host and its subdomains should be allowed")
	}
	if client.hostAllowed("images.example.com.attacker.test") {
		t.Fatal("suffix lookalike must not be allowed")
	}
}

func TestValidateImageAllowsLarge4KPNGWithinLimit(t *testing.T) {
	data := make([]byte, (20<<20)+1)
	copy(data, []byte("\x89PNG\r\n\x1a\n"))
	image, err := validateImage(data)
	if err != nil {
		t.Fatalf("expected a generated image larger than 20 MB to be accepted: %v", err)
	}
	if image.MIME != "image/png" {
		t.Fatalf("unexpected MIME: %s", image.MIME)
	}
}
