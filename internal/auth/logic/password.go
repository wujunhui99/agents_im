package logic

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"

	"github.com/wujunhui99/agents_im/internal/auth/model"
	"golang.org/x/crypto/bcrypt"
)

type PasswordHasher interface {
	Hash(password string) (hash string, salt string, version string, err error)
	Verify(password string, salt string, hash string, version string) bool
}

type BcryptPasswordHasher struct {
	cost   int
	legacy *IterativeSHA256Hasher
}

type IterativeSHA256Hasher struct {
	iterations int
	saltBytes  int
}

func NewPasswordHasher() *BcryptPasswordHasher {
	return &BcryptPasswordHasher{
		cost:   bcrypt.DefaultCost,
		legacy: NewLegacySHA256Hasher(),
	}
}

func NewLegacySHA256Hasher() *IterativeSHA256Hasher {
	return &IterativeSHA256Hasher{
		iterations: 60000,
		saltBytes:  16,
	}
}

func (h *BcryptPasswordHasher) Hash(password string) (string, string, string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", "", "", err
	}
	return string(hash), "", model.PasswordHashVersionBcrypt, nil
}

func (h *BcryptPasswordHasher) Verify(password string, salt string, hash string, version string) bool {
	switch version {
	case model.PasswordHashVersionBcrypt:
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	case model.PasswordHashVersionLegacySHA256:
		return h.legacy.Verify(password, salt, hash, version)
	default:
		return false
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
		model.PasswordHashVersionLegacySHA256,
		nil
}

func (h *IterativeSHA256Hasher) Verify(password string, salt string, hash string, version string) bool {
	if version != model.PasswordHashVersionLegacySHA256 {
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
