package pixian

import (
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRemoveBackgroundUsesBasicAuthAndReturnsHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "api-id" || password != "secret" {
			t.Fatalf("unexpected authentication: %q %q", user, password)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("output.format") != "png" || r.FormValue("test") != "true" || r.FormValue("max_pixels") != "25000000" {
			t.Fatalf("unexpected form values: %#v", r.MultipartForm.Value)
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("X-Credits-Calculated", "0.079")
		w.Header().Set("X-Result-Size", "1 1")
		_ = png.Encode(w, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	}))
	defer server.Close()
	client := NewWithBaseURL(server.URL, "api-id", "secret", 180*time.Second)
	result, err := client.RemoveBackground(context.Background(), "input.png", "image/png", []byte{1}, true, 25_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if result.CreditsCalculated != .079 || result.ResultSize != "1 1" || len(result.Data) == 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
