package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	const plain = "s3cret-password!"
	hash, err := HashPassword(plain)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == plain {
		t.Fatal("hash must not equal plain text")
	}
	if !VerifyPassword(hash, plain) {
		t.Fatal("VerifyPassword should accept the correct password")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("VerifyPassword must reject a wrong password")
	}
}

func TestHashPasswordRejectsOverlongInput(t *testing.T) {
	// bcrypt only uses the first 72 bytes; longer input must be an error
	// rather than silent truncation.
	if _, err := HashPassword(strings.Repeat("a", 73)); err == nil {
		t.Fatal("HashPassword should reject passwords longer than 72 bytes")
	}
}

func TestVerifyPasswordMalformedHash(t *testing.T) {
	if VerifyPassword("not-a-bcrypt-hash", "whatever") {
		t.Fatal("VerifyPassword must return false for a malformed hash")
	}
}
