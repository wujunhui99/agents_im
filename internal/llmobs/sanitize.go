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

var sensitiveInlinePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._~+/=-]+`),
	regexp.MustCompile(`(?i)((?:api[_-]?key|access[_-]?token|refresh[_-]?token|token|password|secret|cookie|dsn)=)[^\s&]+`),
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
		value = pattern.ReplaceAllString(value, "${1}[REDACTED]")
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
