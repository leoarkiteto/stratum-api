package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/stratum/internal/shared/httputil"
	"github.com/leoarkiteto/stratum/internal/shared/session"
)

// fakeSessionStore is an in-memory session.Store for handler tests.
type fakeSessionStore struct {
	sessions map[string]*session.Session
	nextID   int64
}

func newFakeSessionStore() *fakeSessionStore {
	return &fakeSessionStore{sessions: map[string]*session.Session{}, nextID: 1}
}

func (f *fakeSessionStore) Create(_ context.Context) (string, string, error) {
	token, csrf := fmt.Sprintf("tok-%d", f.nextID), fmt.Sprintf("csrf-%d", f.nextID)
	f.nextID++
	f.sessions[token] = &session.Session{CSRF: csrf}
	return token, csrf, nil
}

func (f *fakeSessionStore) Get(_ context.Context, token string) (*session.Session, error) {
	s, ok := f.sessions[token]
	if !ok {
		return nil, session.ErrNotFound
	}
	return s, nil
}

func (f *fakeSessionStore) BindUser(_ context.Context, token string, userID int64) (string, error) {
	s, ok := f.sessions[token]
	if !ok {
		return "", session.ErrNotFound
	}
	newToken := fmt.Sprintf("tok-%d", f.nextID)
	f.nextID++
	s.UserID = userID
	delete(f.sessions, token)
	f.sessions[newToken] = s
	return newToken, nil
}

func (f *fakeSessionStore) Delete(_ context.Context, token string) error {
	delete(f.sessions, token)
	return nil
}

func newTestMux() (http.Handler, *fakeSessionStore, *fakeUserStore) {
	fs := newFakeSessionStore()
	us := newFakeUserStore()
	h := New(Deps{Users: us, Sessions: fs, Hasher: newTestHasher(), Secure: false, SessionTTL: time.Hour})
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux, fs, us
}

func seedUser(t *testing.T, us *fakeUserStore) int64 {
	t.Helper()
	svc := NewService(us, newTestHasher())
	u, err := svc.Register(context.Background(), RegisterParams{
		Name: "Ana Souza", Email: "ana@example.com", Password: "strong-password",
	})
	if err != nil {
		t.Fatalf("seed Register: %v", err)
	}
	return u.ID
}

func doForm(h http.Handler, method, path string, form url.Values, cookie *http.Cookie, htmx bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if htmx {
		req.Header.Set("HX-Request", "true")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestLoginPageRenders(t *testing.T) {
	mux, _, _ := newTestMux()
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest("GET", "/login", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Log in") {
		t.Error("login page should contain the form")
	}
	if !strings.Contains(rr.Body.String(), `name="csrf"`) {
		t.Error("login form must include a csrf hidden field")
	}
	if len(rr.Result().Cookies()) == 0 {
		t.Error("login page must set a session cookie")
	}
}

func TestLoginSuccessRedirects(t *testing.T) {
	mux, fs, us := newTestMux()
	seedUser(t, us)
	token, csrf, _ := fs.Create(context.Background())

	rr := doForm(mux, "POST", "/login", url.Values{
		"csrf": {csrf}, "email": {"ana@example.com"}, "password": {"strong-password"},
	}, &http.Cookie{Name: httputil.SessionCookieName, Value: token}, false)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body: %s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "/dashboard" {
		t.Errorf("Location = %q, want /dashboard", loc)
	}
	if len(rr.Result().Cookies()) == 0 {
		t.Error("login must set a fresh session cookie (token rotation)")
	}
}

func TestLoginWrongPasswordForm(t *testing.T) {
	mux, fs, us := newTestMux()
	seedUser(t, us)
	token, csrf, _ := fs.Create(context.Background())

	rr := doForm(mux, "POST", "/login", url.Values{
		"csrf": {csrf}, "email": {"ana@example.com"}, "password": {"wrong"},
	}, &http.Cookie{Name: httputil.SessionCookieName, Value: token}, false)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 re-render", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Invalid email or password.") {
		t.Error("wrong password should render an error")
	}
}

func TestLoginHTMXRedirects(t *testing.T) {
	mux, fs, us := newTestMux()
	seedUser(t, us)
	token, csrf, _ := fs.Create(context.Background())

	rr := doForm(mux, "POST", "/login", url.Values{
		"csrf": {csrf}, "email": {"ana@example.com"}, "password": {"strong-password"},
	}, &http.Cookie{Name: httputil.SessionCookieName, Value: token}, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for htmx", rr.Code)
	}
	if loc := rr.Header().Get("HX-Redirect"); loc != "/dashboard" {
		t.Errorf("HX-Redirect = %q, want /dashboard", loc)
	}
}

func TestLoginRejectsWrongCSRF(t *testing.T) {
	mux, fs, us := newTestMux()
	seedUser(t, us)
	token, _, _ := fs.Create(context.Background())

	rr := doForm(mux, "POST", "/login", url.Values{
		"csrf": {"bogus"}, "email": {"ana@example.com"}, "password": {"strong-password"},
	}, &http.Cookie{Name: httputil.SessionCookieName, Value: token}, false)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "session expired") {
		t.Error("wrong csrf should re-render with a session error")
	}
}

func TestRegisterSuccess(t *testing.T) {
	mux, fs, _ := newTestMux()
	token, csrf, _ := fs.Create(context.Background())

	rr := doForm(mux, "POST", "/register", url.Values{
		"csrf": {csrf}, "name": {"Ana Souza"}, "email": {"ana@example.com"},
		"password": {"strong-password"}, "role": {"owner"},
	}, &http.Cookie{Name: httputil.SessionCookieName, Value: token}, false)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body: %s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "/dashboard" {
		t.Errorf("Location = %q, want /dashboard", loc)
	}
}

func TestRegisterDuplicateEmailForm(t *testing.T) {
	mux, fs, us := newTestMux()
	seedUser(t, us)
	token, csrf, _ := fs.Create(context.Background())

	rr := doForm(mux, "POST", "/register", url.Values{
		"csrf": {csrf}, "name": {"Ana Souza"}, "email": {"ana@example.com"},
		"password": {"strong-password"}, "role": {"owner"},
	}, &http.Cookie{Name: httputil.SessionCookieName, Value: token}, false)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 re-render", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "already exists") {
		t.Error("duplicate email should render an error")
	}
}

func TestRegisterValidationError(t *testing.T) {
	mux, fs, _ := newTestMux()
	token, csrf, _ := fs.Create(context.Background())

	rr := doForm(mux, "POST", "/register", url.Values{
		"csrf": {csrf}, "name": {"Ana"}, "email": {"ana@example.com"},
		"password": {"short"}, "role": {"owner"},
	}, &http.Cookie{Name: httputil.SessionCookieName, Value: token}, false)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "at least 8 characters") {
		t.Error("short password should render a validation error")
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	mux, _, _ := newTestMux()

	rr := doForm(mux, "POST", "/logout", nil,
		&http.Cookie{Name: httputil.SessionCookieName, Value: "whatever"}, false)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rr.Code)
	}
	cleared := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == httputil.SessionCookieName {
			if c.MaxAge >= 0 {
				t.Error("logout must clear the session cookie")
			}
			cleared = true
		}
	}
	if !cleared {
		t.Error("logout must set a clearing cookie")
	}
}
