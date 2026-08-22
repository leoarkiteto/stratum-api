package password

import (
	"strings"
	"testing"
)

const testPepper = "a-super-secret-pepper-that-is-at-least-32-characters-long"

func TestHashAndVerifyRoundtrip(t *testing.T) {
	h := New(testPepper, DefaultParams)
	hash, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !h.Verify(hash, "correct horse battery staple") {
		t.Fatal("Verify should accept the correct password")
	}
	if h.Verify(hash, "wrong password") {
		t.Fatal("Verify must reject a wrong password")
	}
}

func TestHashProducesPHCFormat(t *testing.T) {
	h := New(testPepper, DefaultParams)
	hash, err := h.Hash("s3cret-password!")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=1,p=4$") {
		t.Errorf("hash does not follow the PHC format: %q", hash)
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Fatalf("expected 6 $ segments, got %d: %q", len(parts), hash)
	}
	if parts[4] == "" || parts[5] == "" {
		t.Error("salt and hash segments must be non-empty")
	}
}

func TestHashIsSalted(t *testing.T) {
	h := New(testPepper, DefaultParams)
	a, _ := h.Hash("same-password")
	b, _ := h.Hash("same-password")
	if a == b {
		t.Fatal("two hashes of the same password must differ (random salt)")
	}
}

func TestPepperChangesHashAndIsNotStored(t *testing.T) {
	plain := "same-password"
	h1 := New(testPepper, DefaultParams)
	h2 := New("another-secret-pepper-01234567890123456789", DefaultParams)

	a, err := h1.Hash(plain)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	b, err := h2.Hash(plain)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	// The pepper must never appear in the encoded string.
	if strings.Contains(a, testPepper) {
		t.Fatal("encoded hash must never contain the pepper")
	}
	// A hash made with one pepper must not verify against another.
	if h2.Verify(a, plain) {
		t.Fatal("a hash must not verify with a different pepper")
	}
	if h1.Verify(b, plain) {
		t.Fatal("a hash must not verify with a different pepper")
	}
}

func TestVerifyUsesParametersFromStoredHash(t *testing.T) {
	// Hash with non-default (lighter) parameters; verification must succeed by
	// reading the parameters out of the stored string, not the defaults.
	light := Params{Memory: 8 * 1024, Iterations: 2, Parallelism: 1, SaltLength: 16, KeyLength: 32}
	h := New(testPepper, light)
	hash, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	// A verifier configured with the defaults must still accept it.
	def := New(testPepper, DefaultParams)
	if !def.Verify(hash, "correct horse battery staple") {
		t.Fatal("Verify must honor the parameters stored in the hash")
	}
	if def.Verify(hash, "wrong") {
		t.Fatal("Verify must reject a wrong password")
	}
}

func TestVerifyRejectsMalformedHashes(t *testing.T) {
	h := New(testPepper, DefaultParams)
	for _, bad := range []string{
		"",
		"not-a-hash",
		"$bcrypt$v=2$m=65536,t=1,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=18$m=65536,t=1,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=0,t=1,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=65536,t=0,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=65536,t=1,p=0$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=9999999999,t=1,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=65536,t=999,t=1,p=4$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=65536,t=1,p=4$!!!$aGFzaA",
		"$argon2id$v=19$m=65536,t=1,p=4$c2FsdA$!!!",
		"$argon2id$v=19$m=65536,t=1,p=4$$aGFzaA",
	} {
		if h.Verify(bad, "whatever") {
			t.Errorf("Verify(%q) should return false", bad)
		}
	}
}

func TestVerifyRejectsTamperedHash(t *testing.T) {
	h := New(testPepper, DefaultParams)
	hash, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	// Flip the last character of the key segment.
	if h.Verify(hash[:len(hash)-1]+"x", "correct horse battery staple") {
		t.Fatal("Verify must reject a tampered hash")
	}
}

func TestDecodeRoundtrip(t *testing.T) {
	h := New(testPepper, DefaultParams)
	hash, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	p, salt, key, err := decode(hash)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Memory != DefaultParams.Memory || p.Iterations != DefaultParams.Iterations || p.Parallelism != DefaultParams.Parallelism {
		t.Errorf("decoded params = %+v, want %+v", p, DefaultParams)
	}
	if len(salt) != int(DefaultParams.SaltLength) || len(key) != int(DefaultParams.KeyLength) {
		t.Errorf("decoded salt/key lengths = %d/%d", len(salt), len(key))
	}
}
