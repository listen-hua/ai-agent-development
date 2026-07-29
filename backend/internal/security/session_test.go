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

func TestIAMSessionRoundTrip(t *testing.T) {
	sessions := NewSessions("a-secret-long-enough-for-tests")
	token, err := sessions.IssueFor("user-1", "iam", 18, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := sessions.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.AuthSource != "iam" || claims.IAMUserID != 18 {
		t.Fatalf("unexpected IAM claims: %#v", claims)
	}
}
