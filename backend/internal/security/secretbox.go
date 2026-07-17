package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

type SecretBox struct {
	aead cipher.AEAD
}

func NewSecretBox(secret string) (*SecretBox, error) {
	if len(secret) < 16 {
		return nil, errors.New("secret encryption key must be at least 16 characters")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &SecretBox{aead: aead}, nil
}

func (b *SecretBox) Encrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(value), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (b *SecretBox) Decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	sealed, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	nonceSize := b.aead.NonceSize()
	if len(sealed) <= nonceSize {
		return "", errors.New("encrypted secret is invalid")
	}
	returnValue, err := b.aead.Open(nil, sealed[:nonceSize], sealed[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(returnValue), nil
}
