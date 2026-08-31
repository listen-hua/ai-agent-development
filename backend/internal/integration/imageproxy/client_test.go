package imageproxy

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"testing"
	"time"
)

func TestGPTImage2GenerationInputUsesConcretePixelSize(t *testing.T) {
	input := gptImage2GenerationInput("gpt-image-2", "poster", "2048x1152")
	if input["size"] != "2048x1152" {
		t.Fatalf("unexpected GPT Image 2 input: %#v", input)
	}
	if _, exists := input["resolution"]; exists {
		t.Fatal("GPT Image 2 must not send the obsolete semantic resolution field")
	}
	if _, exists := input["extra_body"]; exists {
		t.Fatal("GPT Image 2 must not use Gemini extra_body")
	}
}

func TestGPTImage2EditBodyIncludesReferencesAndConcreteSize(t *testing.T) {
	body, contentType, err := gptImage2EditBody("gpt-image-2", "edit", "3840x2160", []ReferenceImage{
		{FileName: "one.png", MIME: "image/png", Data: []byte("one")},
		{FileName: "two.webp", MIME: "image/webp", Data: []byte("two")},
	})
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(body, strings.TrimPrefix(contentType, "multipart/form-data; boundary="))
	fields := map[string]string{}
	images := 0
	for {
		part, readErr := reader.NextPart()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		data, _ := io.ReadAll(part)
		if part.FormName() == "image" {
			images++
		} else {
			fields[part.FormName()] = string(data)
		}
	}
	if images != 2 || fields["size"] != "3840x2160" {
		t.Fatalf("unexpected edit form: images=%d fields=%#v", images, fields)
	}
	if _, exists := fields["resolution"]; exists {
		t.Fatalf("GPT Image 2 edits must use concrete size rather than resolution: %#v", fields)
	}
	if _, exists := fields["quality"]; exists {
		t.Fatalf("GPT Image 2 resolution tiers must not be sent as quality: %#v", fields)
	}
}

func TestGPTImagePixelSizesAcrossRatiosAndTiers(t *testing.T) {
	tests := []struct{ ratio, tier, want string }{
		{"1:1", "1K", "1024x1024"}, {"16:9", "1K", "1280x720"},
		{"4:3", "1K", "1152x864"}, {"1:1", "2K", "2048x2048"},
		{"16:9", "2K", "2048x1152"}, {"1:1", "4K", "2880x2880"},
		{"16:9", "4K", "3840x2160"}, {"4:3", "4K", "3264x2448"},
	}
	for _, test := range tests {
		got, err := GPTImagePixelSize(test.ratio, test.tier, nil)
		if err != nil || got != test.want {
			t.Fatalf("%s %s: got %q (%v), want %q", test.ratio, test.tier, got, err, test.want)
		}
	}
}

func TestGPTImageOriginalRatioUsesFirstReference(t *testing.T) {
	got, err := GPTImagePixelSize("original", "2K", []ReferenceImage{{Width: 1600, Height: 900}, {Width: 900, Height: 1600}})
	if err != nil {
		t.Fatal(err)
	}
	if got != "2720x1536" {
		t.Fatalf("unexpected original-ratio size: %s", got)
	}
	if _, err = GPTImagePixelSize("original", "2K", []ReferenceImage{{Width: 4000, Height: 1000}}); err == nil {
		t.Fatal("references wider than 3:1 must be rejected")
	}
}

func TestUniversalImageEditBodyUsesXGAPIFields(t *testing.T) {
	body, contentType, err := universalImageEditBody("gpt-image-2", "poster", "16:9", "2K", []ReferenceImage{
		{FileName: "reference.png", MIME: "image/png", Data: []byte("reference")},
	})
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(body, strings.TrimPrefix(contentType, "multipart/form-data; boundary="))
	fields := map[string]string{}
	images := 0
	for {
		part, readErr := reader.NextPart()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		data, _ := io.ReadAll(part)
		if part.FormName() == "image" {
			images++
		} else {
			fields[part.FormName()] = string(data)
		}
	}
	if images != 1 || fields["aspect_ratio"] != "16:9" || fields["quality"] != "2K" || fields["response_format"] != "b64_json" {
		t.Fatalf("unexpected universal edit form: images=%d fields=%#v", images, fields)
	}
	if _, exists := fields["resolution"]; exists {
		t.Fatalf("XGAPI universal edits must use quality rather than resolution: %#v", fields)
	}
}

func TestUniversalImageEditBodyMatchesDocumentedPartOrderAndBoundsHeaders(t *testing.T) {
	largeReference := make([]byte, (2<<20)+17)
	copy(largeReference, []byte("\x89PNG\r\n\x1a\n"))
	longUnsafeName := strings.Repeat("very-long-name", 1000) + "\r\nInjected: value.png"
	body, contentType, err := universalImageEditBody(
		"gemini-3.1-flash-image-preview-2k",
		strings.Repeat("详细描述", 2000),
		"1:1",
		"2K",
		[]ReferenceImage{{FileName: longUnsafeName, MIME: "image/png", Data: largeReference}},
	)
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(body, strings.TrimPrefix(contentType, "multipart/form-data; boundary="))
	partNames := []string{}
	for {
		part, readErr := reader.NextPart()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			t.Fatalf("large XGAPI multipart body must remain parseable: %v", readErr)
		}
		partNames = append(partNames, part.FormName())
		if part.FormName() == "image" && part.FileName() != "reference-1.png" {
			t.Fatalf("unsafe source filename leaked into multipart header: %q", part.FileName())
		}
		if _, err = io.Copy(io.Discard, part); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"model", "prompt", "image", "aspect_ratio", "quality", "response_format"}
	if strings.Join(partNames, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected XGAPI multipart order: got %v want %v", partNames, want)
	}
}

func TestMultipartInvalidRequestIsNotRetried(t *testing.T) {
	err := &APIError{StatusCode: 500, Body: `{"error":{"message":"multipart: NextPart: bufio: buffer full","code":"invalid_request"}}`}
	if err.Retryable() {
		t.Fatal("deterministic multipart parse failures must not be retried")
	}
	if !(&APIError{StatusCode: 500, Body: `{"error":"temporary"}`}).Retryable() {
		t.Fatal("ordinary relay 500 errors should remain retryable")
	}
}

func TestUnavailableModelChannelIsNotRetried(t *testing.T) {
	err := &APIError{StatusCode: 503, Body: `{"error":{"message":"No available channel for model demo under group default","code":"model_not_found"}}`}
	if err.Retryable() {
		t.Fatal("an unavailable model channel cannot recover by replaying the same paid request")
	}
}

func TestEndpointTypesAreNormalized(t *testing.T) {
	got := normalizedEndpointTypes([]string{" OpenAI ", "image-generation", "OPENAI", ""})
	want := []string{"image-generation", "openai"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected endpoint types: got %v want %v", got, want)
	}
}

func TestUniversalImageEditBodySupportsTextOnlyGeneration(t *testing.T) {
	body, contentType, err := universalImageEditBody("gpt-image-2", "poster", "1:1", "1K", nil)
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(body, strings.TrimPrefix(contentType, "multipart/form-data; boundary="))
	images := 0
	for {
		part, readErr := reader.NextPart()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		if part.FormName() == "image" {
			images++
		}
	}
	if images != 0 {
		t.Fatalf("text-only generation must omit image parts, got %d", images)
	}
}

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

func TestChatGenerationInputOmitsOriginalAspectRatio(t *testing.T) {
	input := chatGenerationInput("gemini-3.1-flash-image", "edit this image", "original", "2K", []string{"data:image/png;base64,eA=="})
	extraBody := input["extra_body"].(map[string]any)
	google := extraBody["google"].(map[string]any)
	config := google["image_config"].(map[string]string)
	if _, exists := config["aspect_ratio"]; exists {
		t.Fatalf("original ratio must omit aspect_ratio: %#v", config)
	}
	if config["image_size"] != "2K" {
		t.Fatalf("image_size must still be sent: %#v", config)
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
		{ratio: "4:3", tier: "4K", width: 4608, height: 3456},
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
