package logic

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

const passwordHashVersion = "sha256-iter-v1"

type PasswordHasher interface {
	Hash(password string) (hash string, salt string, version string, err error)
	Verify(password string, salt string, hash string, version string) bool
}

type IterativeSHA256Hasher struct {
	iterations int
	saltBytes  int
}

func NewPasswordHasher() *IterativeSHA256Hasher {
	return &IterativeSHA256Hasher{
		iterations: 60000,
		saltBytes:  16,
	}
}

func (h *IterativeSHA256Hasher) Hash(password string) (string, string, string, error) {
	salt := make([]byte, h.saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", "", "", err
	}

	digest := h.derive(password, salt)
	return base64.RawURLEncoding.EncodeToString(digest),
		base64.RawURLEncoding.EncodeToString(salt),
		passwordHashVersion,
		nil
}

func (h *IterativeSHA256Hasher) Verify(password string, salt string, hash string, version string) bool {
	if version != passwordHashVersion {
		return false
	}

	saltBytes, err := base64.RawURLEncoding.DecodeString(salt)
	if err != nil {
		return false
	}
	expectedHash, err := base64.RawURLEncoding.DecodeString(hash)
	if err != nil {
		return false
	}

	actualHash := h.derive(password, saltBytes)
	return hmac.Equal(actualHash, expectedHash)
}

func (h *IterativeSHA256Hasher) derive(password string, salt []byte) []byte {
	hash := sha256.New()
	_, _ = hash.Write(salt)
	_, _ = hash.Write([]byte(password))
	digest := hash.Sum(nil)

	for i := 1; i < h.iterations; i++ {
		hash.Reset()
		_, _ = hash.Write(salt)
		_, _ = hash.Write(digest)
		_, _ = hash.Write([]byte(password))
		digest = hash.Sum(nil)
	}

	return digest
}
