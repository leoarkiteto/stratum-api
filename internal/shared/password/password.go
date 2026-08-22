// Package password implements Argon2id password hashing with an HMAC-SHA256
// pepper applied before hashing.
//
// The pepper is a server-side secret read from configuration; it is used as
// the key of an HMAC-SHA256 over the plain-text password, and the resulting
// digest is fed into Argon2id. The pepper is deliberately never stored in the
// encoded hash string, so a database leak alone is not enough to brute-force
// passwords offline.
package password

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params holds the tunable Argon2id parameters.
type Params struct {
	Memory      uint32 // KiB of memory to use
	Iterations  uint32 // number of passes
	Parallelism uint8  // number of lanes/threads
	SaltLength  uint32 // salt size in bytes
	KeyLength   uint32 // output key size in bytes
}

// DefaultParams are the OWASP-recommended Argon2id parameters.
// 64 MiB memory, 1 iteration, 4 lanes, 16-byte salt, 32-byte key.
var DefaultParams = Params{
	Memory:      64 * 1024,
	Iterations:  1,
	Parallelism: 4,
	SaltLength:  16,
	KeyLength:   32,
}

// DoS guards for parameters parsed from a stored hash. They bound how much
// work a maliciously crafted hash can force a verification to perform.
const (
	maxMemory     = 1 << 20 // 1 GiB (KiB)
	maxIterations = 10
)

// Hasher hashes and verifies passwords with Argon2id and a secret pepper.
type Hasher struct {
	pepper []byte
	params Params
}

// New builds a Hasher. pepper must be a non-empty server-side secret (enforced
// by config) and is never stored in the encoded hash.
func New(pepper string, params Params) *Hasher {
	return &Hasher{pepper: []byte(pepper), params: params}
}

// Hash returns the encoded Argon2id hash for plain, in PHC string format:
//
//	$argon2id$v=19$m=65536,t=1,p=4$<b64-salt>$<b64-hash>
func (h *Hasher) Hash(plain string) (string, error) {
	salt := make([]byte, h.params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := h.derive(plain, salt, h.params)
	return encode(key, salt, h.params), nil
}

// Verify reports whether plain matches the encoded hash. The Argon2id
// parameters are read from the stored hash (not the configured defaults), and
// the final comparison uses a constant-time operation to avoid timing leaks.
func (h *Hasher) Verify(encoded, plain string) bool {
	params, salt, key, err := decode(encoded)
	if err != nil {
		return false
	}
	other := h.derive(plain, salt, params)
	return subtle.ConstantTimeCompare(key, other) == 1
}

// derive applies HMAC-SHA256(pepper, plain) and then Argon2id.
func (h *Hasher) derive(plain string, salt []byte, p Params) []byte {
	return argon2.IDKey(h.pepperKey(plain), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
}

// pepperKey returns HMAC-SHA256(pepper, plain) — the pepper is the HMAC key.
func (h *Hasher) pepperKey(plain string) []byte {
	mac := hmac.New(sha256.New, h.pepper)
	mac.Write([]byte(plain))
	return mac.Sum(nil)
}

// encode formats key and salt in the PHC-style argon2id string.
func encode(key, salt []byte, p Params) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

// decode parses an encoded hash into its parameters, salt and key.
func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, errors.New("password: malformed hash")
	}
	if v := strings.TrimPrefix(parts[2], "v="); v != strconv.Itoa(int(argon2.Version)) {
		return Params{}, nil, nil, errors.New("password: unsupported argon2 version")
	}

	var memory, iterations, parallelism uint64
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return Params{}, nil, nil, errors.New("password: malformed parameters")
	}
	if memory == 0 || memory > maxMemory || iterations == 0 || iterations > maxIterations || parallelism == 0 || parallelism > 255 {
		return Params{}, nil, nil, errors.New("password: invalid parameters")
	}
	p := Params{
		Memory:      uint32(memory),
		Iterations:  uint32(iterations),
		Parallelism: uint8(parallelism),
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, errors.New("password: malformed salt")
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, errors.New("password: malformed hash")
	}
	if len(salt) == 0 || len(key) == 0 {
		return Params{}, nil, nil, errors.New("password: empty salt or hash")
	}
	p.SaltLength = uint32(len(salt))
	p.KeyLength = uint32(len(key))
	return p, salt, key, nil
}
