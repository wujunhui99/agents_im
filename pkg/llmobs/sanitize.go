package llmobs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

const maxErrorMessageLength = 512

type redactionPattern struct {
	pattern     *regexp.Regexp
	replacement string
}

var sensitiveInlinePatterns = []redactionPattern{
	{regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._~+/=-]+`), "${1}[REDACTED]"},
	{regexp.MustCompile(`(?i)((?:api[_-]?key|access[_-]?token|refresh[_-]?token|token|password|secret|cookie|dsn)=)[^\s&]+`), "${1}[REDACTED]"},
	{regexp.MustCompile(`(?i)\b((?:postgres(?:ql)?|mysql|redis|mongodb(?:\+srv)?|amqp|kafka)://)[^\s]+`), "${1}[REDACTED]"},
	{regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*\b`), "[REDACTED]"},
	{regexp.MustCompile(`(?i)\bsk-[a-z0-9][a-z0-9._-]{10,}\b`), "[REDACTED]"},
}

func ErrorFields(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	return errorClass(err), RedactPlainText(err.Error())
}

func RedactPlainText(value string) string {
	value = strings.TrimSpace(value)
	for _, pattern := range sensitiveInlinePatterns {
		value = pattern.pattern.ReplaceAllString(value, pattern.replacement)
	}
	if len(value) <= maxErrorMessageLength {
		return value
	}
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("sha256:%s size_bytes:%d", hex.EncodeToString(sum[:]), len([]byte(value)))
}

func TextSummary(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("sha256:%s size_bytes:%d", hex.EncodeToString(sum[:]), len([]byte(value)))
}

func PromptHash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func errorClass(err error) string {
	typ := reflect.TypeOf(err)
	if typ == nil {
		return "error"
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Name() == "" {
		return "error"
	}
	return typ.Name()
}
