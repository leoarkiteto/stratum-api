// Package auth is responsable to implement the JWT strategy
// and add middleware to all request
package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/leoarkiteto/stratum-api/internal/model"
)

// JWTManager issues and parses HS256-signed access tokens.
type JWTManager struct {
	issuer string
	secret []byte
	ttl    time.Duration
}

// NewJWTManager builds a JWTManager. The secret must be kept secret.
func NewJWTManager(secret string, ttl time.Duration, issuer string) *JWTManager {
	return &JWTManager{secret: []byte(secret), ttl: ttl, issuer: issuer}
}

// Claims are the JWT payload. We keep them minimal: sub (user id),
// email and role. Anything else can be loaded from the DB when needed.
type Claims struct {
	Email string     `json:"email"`
	Role  model.Role `json:"role"`
	jwt.RegisteredClaims
}

// Issue creates a signed token for the given user.
func (m *JWTManager) Issue(u *model.User) (string, error) {
	now := time.Now()
	claims := Claims{
		Email: u.Email,
		Role:  u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(u.ID, 10),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// Parse validates a token and returns its claims.
// It enforces the signing method (HS256 only) and the expected issuer.
func (m *JWTManager) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %q", t.Method.Alg())
		}
		return m.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
