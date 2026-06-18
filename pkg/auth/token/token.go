package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

type Claims struct {
	UserID     string
	Identifier string
	JTI        string
	Device     string
	LoginIP    string
	IssuedAt   time.Time
	ExpiresAt  time.Time
}

type Manager interface {
	Issue(userID string, identifier string, device string, loginIP string) (string, Claims, error)
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

func (m *HMACTokenManager) Issue(userID string, identifier string, device string, loginIP string) (string, Claims, error) {
	userID = strings.TrimSpace(userID)
	identifier = strings.TrimSpace(identifier)
	if userID == "" {
		return "", Claims{}, apperror.InvalidArgument("user_id is required")
	}
	if identifier == "" {
		return "", Claims{}, apperror.InvalidArgument("identifier is required")
	}

	issuedAt := m.now().UTC()
	claims := Claims{
		UserID:     userID,
		Identifier: identifier,
		JTI:        uuid.NewString(),
		Device:     strings.TrimSpace(device),
		LoginIP:    strings.TrimSpace(loginIP),
		IssuedAt:   issuedAt,
		ExpiresAt:  issuedAt.Add(m.ttl),
	}

	headerSegment, err := encodeJSON(tokenHeader{Algorithm: "HS256", Type: "JWT"})
	if err != nil {
		return "", Claims{}, err
	}
	payloadSegment, err := encodeJSON(tokenPayload{
		Subject:    claims.UserID,
		UserID:     claims.UserID,
		Identifier: claims.Identifier,
		JTI:        claims.JTI,
		SessionID:  claims.JTI,
		Device:     claims.Device,
		LoginIP:    claims.LoginIP,
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
	if strings.TrimSpace(payload.Subject) == "" || strings.TrimSpace(payload.Identifier) == "" || payload.ExpiresAt <= 0 {
		return Claims{}, apperror.Unauthenticated("token payload is invalid")
	}

	return Claims{
		UserID:     payload.Subject,
		Identifier: payload.Identifier,
		JTI:        payload.JTI,
		Device:     payload.Device,
		LoginIP:    payload.LoginIP,
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
	// sub and jti are the standard JWT claims. go-zero's jwt middleware strips the
	// registered claims (sub/jti/iat/exp/...) from the request context, so each
	// value the DeviceAuth middleware / handlers need is mirrored under a
	// non-registered key (user_id / session_id / device_type) that go-zero copies.
	Subject    string `json:"sub"`
	UserID     string `json:"user_id"`
	Identifier string `json:"identifier"`
	JTI        string `json:"jti"`
	SessionID  string `json:"session_id"`
	Device     string `json:"device_type,omitempty"`
	LoginIP    string `json:"login_ip,omitempty"`
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
