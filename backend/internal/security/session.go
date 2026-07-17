package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Claims struct {
	UserID    string `json:"user_id"`
	ExpiresAt int64  `json:"exp"`
}
type Sessions struct{ secret []byte }

func NewSessions(secret string) *Sessions { return &Sessions{secret: []byte(secret)} }
func (s *Sessions) Issue(userID string, ttl time.Duration) (string, error) {
	payload, err := json.Marshal(Claims{UserID: userID, ExpiresAt: time.Now().Add(ttl).Unix()})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func (s *Sessions) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Claims{}, errors.New("invalid session")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, errors.New("invalid signature")
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return Claims{}, errors.New("invalid signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, err
	}
	var claims Claims
	if err = json.Unmarshal(payload, &claims); err != nil {
		return claims, err
	}
	if claims.ExpiresAt < time.Now().Unix() {
		return claims, errors.New("session expired")
	}
	return claims, nil
}
