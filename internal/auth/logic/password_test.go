package logic

import (
	"strings"
	"testing"
)

func TestPasswordHasherHashesWithBcryptCurrentAlgorithm(t *testing.T) {
	hasher := NewPasswordHasher()

	hash, salt, version, err := hasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if version != "bcrypt-v1" {
		t.Fatalf("hash version = %q, want bcrypt-v1", version)
	}
	if salt != "" {
		t.Fatalf("bcrypt salt field = %q, want empty because bcrypt embeds salt in the hash", salt)
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("hash does not look like bcrypt: %q", hash)
	}
	if !hasher.Verify("correct-password", salt, hash, version) {
		t.Fatalf("bcrypt hash did not verify with the original password")
	}
	if hasher.Verify("wrong-password", salt, hash, version) {
		t.Fatalf("bcrypt hash verified with the wrong password")
	}
}

func TestPasswordHasherVerifiesLegacySHA256Hashes(t *testing.T) {
	legacyHasher := &IterativeSHA256Hasher{
		iterations: 60000,
		saltBytes:  16,
	}
	hash, salt, version, err := legacyHasher.Hash("correct-password")
	if err != nil {
		t.Fatalf("legacy hash: %v", err)
	}

	hasher := NewPasswordHasher()
	if !hasher.Verify("correct-password", salt, hash, version) {
		t.Fatalf("legacy sha256 hash with stored salt did not verify")
	}
	if hasher.Verify("wrong-password", salt, hash, version) {
		t.Fatalf("legacy sha256 hash verified with the wrong password")
	}
}
