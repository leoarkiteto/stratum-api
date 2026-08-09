package auth

import (
	"testing"
	"time"

	"github.com/leoarkiteto/stratum-backend/internal/model"
)

func newTestManager(ttl time.Duration) *JWTManager {
	return NewJWTManager("test-secret", ttl, "stratum-test")
}

func TestIssueAndParseRoundtrip(t *testing.T) {
	m := newTestManager(time.Hour)
	u := &model.User{ID: 42, Email: "resident@example.com", Role: model.RoleOwner}

	token, err := m.Issue(u)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	claims, err := m.Parse(token)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.Subject != "42" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "42")
	}
	if claims.Email != u.Email {
		t.Errorf("Email = %q, want %q", claims.Email, u.Email)
	}
	if claims.Role != model.RoleOwner {
		t.Errorf("Role = %q, want %q", claims.Role, model.RoleOwner)
	}
	if claims.Issuer != "stratum-test" {
		t.Errorf("Issuer = %q, want %q", claims.Issuer, "stratum-test")
	}
	if !claims.ExpiresAt.After(time.Now()) {
		t.Error("token must not be expired")
	}
}

func TestParseRejectsTamperedToken(t *testing.T) {
	m := newTestManager(time.Hour)
	token, err := m.Issue(&model.User{ID: 1, Email: "a@b.com", Role: model.RoleOwner})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// Flip the last character of the signature.
	tampered := token[:len(token)-1] + "x"
	if _, err := m.Parse(tampered); err == nil {
		t.Fatal("Parse must reject a tampered token")
	}
}

func TestParseRejectsExpiredToken(t *testing.T) {
	m := newTestManager(-time.Minute) // already expired at issue time
	token, err := m.Issue(&model.User{ID: 1, Email: "a@b.com", Role: model.RoleOwner})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := m.Parse(token); err == nil {
		t.Fatal("Parse must reject an expired token")
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	issuer := NewJWTManager("secret-a", time.Hour, "stratum-test")
	verifier := NewJWTManager("secret-b", time.Hour, "stratum-test")

	token, err := issuer.Issue(&model.User{ID: 1, Email: "a@b.com", Role: model.RoleOwner})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := verifier.Parse(token); err == nil {
		t.Fatal("Parse must reject a token signed with a different secret")
	}
}

func TestParseRejectsWrongIssuer(t *testing.T) {
	issuer := NewJWTManager("test-secret", time.Hour, "stratum-prod")
	verifier := NewJWTManager("test-secret", time.Hour, "stratum-dev")

	token, err := issuer.Issue(&model.User{ID: 1, Email: "a@b.com", Role: model.RoleOwner})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := verifier.Parse(token); err == nil {
		t.Fatal("Parse must reject a token with the wrong issuer")
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	m := newTestManager(time.Hour)
	if _, err := m.Parse("not-a-jwt"); err == nil {
		t.Fatal("Parse must reject a garbage token")
	}
}
