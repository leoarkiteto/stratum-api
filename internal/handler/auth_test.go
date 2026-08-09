package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/leoarkiteto/stratum-backend/internal/auth"
	"github.com/leoarkiteto/stratum-backend/internal/model"
	"github.com/leoarkiteto/stratum-backend/internal/server"
	"github.com/leoarkiteto/stratum-backend/internal/store"
)

// fakeStore is an in-memory UserStore for handler tests.
type fakeStore struct {
	users   map[int64]*model.User
	byEmail map[string]*model.User
	nextID  int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{users: map[int64]*model.User{}, byEmail: map[string]*model.User{}, nextID: 1}
}

func (f *fakeStore) CreateUser(_ context.Context, u *model.User) (int64, error) {
	if _, exists := f.byEmail[u.Email]; exists {
		return 0, store.ErrEmailTaken
	}
	u.ID = f.nextID
	f.nextID++
	now := time.Now()
	u.CreatedAt, u.UpdatedAt = now, now
	f.users[u.ID] = u
	f.byEmail[u.Email] = u
	return u.ID, nil
}

func (f *fakeStore) GetUserByEmail(_ context.Context, email string) (*model.User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return nil, store.ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) GetUserByID(_ context.Context, id int64) (*model.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return u, nil
}

func newTestServer() (http.Handler, *fakeStore, *auth.JWTManager) {
	fs := newFakeStore()
	jwtm := auth.NewJWTManager("test-secret", time.Hour, "stratum-test")
	mux := server.New(server.Deps{Store: fs, JWT: jwtm})
	return mux, fs, jwtm
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeError(t *testing.T, rr *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var eb errorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &eb); err != nil {
		t.Fatalf("decode error body %q: %v", rr.Body.String(), err)
	}
	return eb
}

func TestRegisterEndpoint(t *testing.T) {
	h, _, _ := newTestServer()
	rr := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"name":     "Ana Souza",
		"email":    "ana@example.com",
		"password": "strong-password",
	}, "")

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Token string     `json:"token"`
		User  model.User `json:"user"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == "" {
		t.Error("response must include a token")
	}
	if resp.User.Role != model.RoleOwner {
		t.Errorf("user.role = %q, want %q", resp.User.Role, model.RoleOwner)
	}
	if resp.User.PasswordHash != "" {
		t.Error("password hash must never be serialized")
	}
}

func TestRegisterInvalidBody(t *testing.T) {
	h, _, _ := newTestServer()
	rr := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{"email": "nope"}, "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if eb := decodeError(t, rr); eb.Error.Code != "invalid_input" {
		t.Errorf("code = %q, want invalid_input", eb.Error.Code)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	h, _, _ := newTestServer()
	payload := map[string]any{"name": "Ana", "email": "ana@example.com", "password": "strong-password"}
	if rr := doJSON(t, h, "POST", "/api/v1/auth/register", payload, ""); rr.Code != 201 {
		t.Fatalf("first register status = %d", rr.Code)
	}
	rr := doJSON(t, h, "POST", "/api/v1/auth/register", payload, "")
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
	if eb := decodeError(t, rr); eb.Error.Code != "email_taken" {
		t.Errorf("code = %q, want email_taken", eb.Error.Code)
	}
}

func TestLoginEndpoint(t *testing.T) {
	h, _, _ := newTestServer()
	doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"name": "Ana", "email": "ana@example.com", "password": "strong-password",
	}, "")

	rr := doJSON(t, h, "POST", "/api/v1/auth/login", map[string]any{
		"email": "ana@example.com", "password": "strong-password",
	}, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "" {
		t.Error("login must return a token")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	h, _, _ := newTestServer()
	doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"name": "Ana", "email": "ana@example.com", "password": "strong-password",
	}, "")

	rr := doJSON(t, h, "POST", "/api/v1/auth/login", map[string]any{
		"email": "ana@example.com", "password": "wrong",
	}, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if eb := decodeError(t, rr); eb.Error.Code != "invalid_credentials" {
		t.Errorf("code = %q, want invalid_credentials", eb.Error.Code)
	}
}

func TestMeRequiresToken(t *testing.T) {
	h, _, _ := newTestServer()
	rr := doJSON(t, h, "GET", "/api/v1/auth/me", nil, "")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if eb := decodeError(t, rr); eb.Error.Code != "unauthorized" {
		t.Errorf("code = %q, want unauthorized", eb.Error.Code)
	}
}

func TestMeRejectsGarbageToken(t *testing.T) {
	h, _, _ := newTestServer()
	rr := doJSON(t, h, "GET", "/api/v1/auth/me", nil, "garbage.token.here")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestMeWithValidToken(t *testing.T) {
	h, _, jwtm := newTestServer()
	rr := doJSON(t, h, "POST", "/api/v1/auth/register", map[string]any{
		"name": "Ana", "email": "ana@example.com", "password": "strong-password",
	}, "")
	if rr.Code != 201 {
		t.Fatalf("register status = %d", rr.Code)
	}
	var reg struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &reg); err != nil {
		t.Fatalf("decode register: %v", err)
	}

	rr = doJSON(t, h, "GET", "/api/v1/auth/me", nil, reg.Token)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	var me model.User
	if err := json.Unmarshal(rr.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.ID != 1 || me.Email != "ana@example.com" {
		t.Errorf("me = %+v, want the registered user", me)
	}

	// A token for a user that no longer exists must yield 404.
	ghostToken, _ := jwtm.Issue(&model.User{ID: 999, Email: "ghost@example.com", Role: model.RoleOwner})
	rr = doJSON(t, h, "GET", "/api/v1/auth/me", nil, ghostToken)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("ghost user status = %d, want 404", rr.Code)
	}
}
