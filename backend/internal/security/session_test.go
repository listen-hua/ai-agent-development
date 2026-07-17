package security

import (
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	sessions := NewSessions("a-secret-long-enough-for-tests")
	token, err := sessions.Issue("user-1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := sessions.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "user-1" {
		t.Fatalf("unexpected user: %s", claims.UserID)
	}
}
func TestSessionRejectsTampering(t *testing.T) {
	sessions := NewSessions("a-secret-long-enough-for-tests")
	token, _ := sessions.Issue("user-1", time.Hour)
	if _, err := sessions.Verify(token + "x"); err == nil {
		t.Fatal("expected tampered token to fail")
	}
}
