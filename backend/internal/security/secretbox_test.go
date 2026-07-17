package security

import "testing"

func TestSecretBoxRoundTrip(t *testing.T) {
	box, err := NewSecretBox("a-long-enough-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := box.Encrypt("gemini-secret-key")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "gemini-secret-key" || encrypted == "" {
		t.Fatal("secret was not encrypted")
	}
	plain, err := box.Decrypt(encrypted)
	if err != nil || plain != "gemini-secret-key" {
		t.Fatalf("unexpected decrypted value: %q, %v", plain, err)
	}
}
