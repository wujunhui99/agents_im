package token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
)

type Claims struct {
	UserID     string
	Identifier string
	SessionID  string
	IssuedAt   time.Time
	ExpiresAt  time.Time
}

type Manager interface {
	Issue(userID string, identifier string) (string, Claims, error)
	Parse(raw string) (Claims, error)
	Validate(raw string) (Claims, error)
}

type HMACTokenManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewHMACTokenManager(secret string, ttl time.Duration) *HMACTokenManager {
	if strings.TrimSpace(secret) == "" {
		secret = "dev-auth-secret-change-me"
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	return &HMACTokenManager{
		secret: []byte(secret),
		ttl:    ttl,
		now:    time.Now,
	}
}

func NewHMACTokenManagerWithClock(secret string, ttl time.Duration, now func() time.Time) *HMACTokenManager {
	manager := NewHMACTokenManager(secret, ttl)
	if now != nil {
		manager.now = now
	}
	return manager
}

func (m *HMACTokenManager) Issue(userID string, identifier string) (string, Claims, error) {
	userID = strings.TrimSpace(userID)
	identifier = strings.TrimSpace(identifier)
	if userID == "" {
		return "", Claims{}, apperror.InvalidArgument("user_id is required")
	}
	if identifier == "" {
		return "", Claims{}, apperror.InvalidArgument("identifier is required")
	}

	issuedAt := m.now().UTC()
	sessionID, err := newSessionID()
	if err != nil {
		return "", Claims{}, err
	}
	claims := Claims{
		UserID:     userID,
		Identifier: identifier,
		SessionID:  sessionID,
		IssuedAt:   issuedAt,
		ExpiresAt:  issuedAt.Add(m.ttl),
	}

	headerSegment, err := encodeJSON(tokenHeader{Algorithm: "HS256", Type: "JWT"})
	if err != nil {
		return "", Claims{}, err
	}
	payloadSegment, err := encodeJSON(tokenPayload{
		UserID:     claims.UserID,
		Identifier: claims.Identifier,
		SessionID:  claims.SessionID,
		IssuedAt:   claims.IssuedAt.Unix(),
		ExpiresAt:  claims.ExpiresAt.Unix(),
	})
	if err != nil {
		return "", Claims{}, err
	}

	signingInput := headerSegment + "." + payloadSegment
	signature := sign(signingInput, m.secret)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), claims, nil
}

func (m *HMACTokenManager) Parse(raw string) (Claims, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Claims{}, apperror.Unauthenticated("token is required")
	}

	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return Claims{}, apperror.Unauthenticated("token format is invalid")
	}

	var header tokenHeader
	if err := decodeJSON(parts[0], &header); err != nil {
		return Claims{}, apperror.Unauthenticated("token header is invalid")
	}
	if header.Algorithm != "HS256" || header.Type != "JWT" {
		return Claims{}, apperror.Unauthenticated("token header is invalid")
	}

	signingInput := parts[0] + "." + parts[1]
	expected := sign(signingInput, m.secret)
	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, apperror.Unauthenticated("token signature is invalid")
	}
	if !hmac.Equal(actual, expected) {
		return Claims{}, apperror.Unauthenticated("token signature is invalid")
	}

	var payload tokenPayload
	if err := decodeJSON(parts[1], &payload); err != nil {
		return Claims{}, apperror.Unauthenticated("token payload is invalid")
	}
	if strings.TrimSpace(payload.UserID) == "" || strings.TrimSpace(payload.Identifier) == "" || payload.ExpiresAt <= 0 {
		return Claims{}, apperror.Unauthenticated("token payload is invalid")
	}

	return Claims{
		UserID:     payload.UserID,
		Identifier: payload.Identifier,
		SessionID:  payload.SessionID,
		IssuedAt:   time.Unix(payload.IssuedAt, 0).UTC(),
		ExpiresAt:  time.Unix(payload.ExpiresAt, 0).UTC(),
	}, nil
}

func (m *HMACTokenManager) Validate(raw string) (Claims, error) {
	claims, err := m.Parse(raw)
	if err != nil {
		return Claims{}, err
	}

	if !m.now().UTC().Before(claims.ExpiresAt) {
		return Claims{}, apperror.Unauthenticated("token expired")
	}

	return claims, nil
}

type tokenHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type tokenPayload struct {
	UserID     string `json:"user_id"`
	Identifier string `json:"identifier"`
	SessionID  string `json:"sid,omitempty"`
	IssuedAt   int64  `json:"iat"`
	ExpiresAt  int64  `json:"exp"`
}

func encodeJSON(value interface{}) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", apperror.Internal("token serialization failed")
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeJSON(segment string, dst interface{}) error {
	raw, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

func sign(signingInput string, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signingInput))
	return mac.Sum(nil)
}

func newSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", apperror.Internal("session id generation failed")
	}
	return "sid_" + hex.EncodeToString(b[:]), nil
}
