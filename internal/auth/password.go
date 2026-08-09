package auth

import "golang.org/x/crypto/bcrypt"

// bcryptCost is the work factor for password hashing.
// DefaultCost (10) is the standard choice; bump only if profiling shows need.
const bcryptCost = bcrypt.DefaultCost

// MaxPasswordBytes is the bcrypt input limit (72 bytes). Longer passwords
// must be rejected up front instead of being silently truncated.
const MaxPasswordBytes = 72

// MinPasswordBytes is the minimum accepted password length.
const MinPasswordBytes = 8

// HashPassword returns a bcrypt hash of the plain-text password.
func HashPassword(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// VerifyPassword reports whether plain matches the stored bcrypt hash.
func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
