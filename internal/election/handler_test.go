package election

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/stratum/internal/model"
	"github.com/leoarkiteto/stratum/internal/session"
	"github.com/leoarkiteto/stratum/internal/web"
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

// fakeUserStore is an in-memory web.UserStore for handler tests.
type fakeUserStore struct {
	users map[int64]*model.User
}

var errUserNotFound = errors.New("user not found")

func (f *fakeUserStore) GetUserByID(_ context.Context, id int64) (*model.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, errUserNotFound
	}
	return u, nil
}

func newTestMux(users ...*model.User) (http.Handler, *fakeSessionStore, *fakeUserStore, *fakeStore) {
	fs := newFakeSessionStore()
	us := &fakeUserStore{users: map[int64]*model.User{}}
	for _, u := range users {
		us.users[u.ID] = u
	}
	es := newFakeStore(users...)
	h := New(Deps{Store: es, Sessions: fs, Users: us, Secure: false})
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux, fs, us, es
}

// loginAs creates a session bound to the user and returns its token + CSRF.
func loginAs(t *testing.T, fs *fakeSessionStore, u *model.User) (string, string) {
	t.Helper()
	token, _, err := fs.Create(context.Background())
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	newToken, err := fs.BindUser(context.Background(), token, u.ID)
	if err != nil {
		t.Fatalf("bind session: %v", err)
	}
	csrf, err := csrfFor(fs, newToken)
	if err != nil {
		t.Fatalf("csrf: %v", err)
	}
	return newToken, csrf
}

func csrfFor(fs *fakeSessionStore, token string) (string, error) {
	s, err := fs.Get(context.Background(), token)
	if err != nil {
		return "", err
	}
	return s.CSRF, nil
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

func doGet(h http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestElectionsRequireAuth(t *testing.T) {
	mux, _, _, _ := newTestMux()
	rr := doGet(mux, "/elections", nil)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
		t.Fatalf("status = %d, location = %q; want 303 /login", rr.Code, rr.Header().Get("Location"))
	}
}

func TestListPageRenders(t *testing.T) {
	syndic, _, _, _ := testUsers()
	mux, fs, _, es := newTestMux(syndic)
	e := &model.Election{Title: "2026 Election", Status: model.ElectionStatusNomination, CreatedBy: syndic.ID}
	if err := es.CreateElection(context.Background(), e); err != nil {
		t.Fatalf("seed election: %v", err)
	}
	token, _ := loginAs(t, fs, syndic)

	rr := doGet(mux, "/elections", &http.Cookie{Name: web.SessionCookieName, Value: token})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Syndic elections") || !strings.Contains(body, "2026 Election") {
		t.Error("list page should show the elections heading and the election title")
	}
	if !strings.Contains(body, "New election") {
		t.Error("list page should show the create button for the syndic")
	}
}

func TestNewPageSyndicOnly(t *testing.T) {
	syndic, owner1, _, _ := testUsers()
	mux, fs, _, _ := newTestMux(syndic, owner1)

	ownerToken, _ := loginAs(t, fs, owner1)
	rr := doGet(mux, "/elections/new", &http.Cookie{Name: web.SessionCookieName, Value: ownerToken})
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/elections" {
		t.Fatalf("owner: status = %d, location = %q; want 303 /elections", rr.Code, rr.Header().Get("Location"))
	}

	syndicToken, _ := loginAs(t, fs, syndic)
	rr = doGet(mux, "/elections/new", &http.Cookie{Name: web.SessionCookieName, Value: syndicToken})
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "New syndic election") {
		t.Fatalf("syndic: status = %d, want 200 with the create form", rr.Code)
	}
}

func TestCreateElectionViaHTTP(t *testing.T) {
	syndic, _, _, _ := testUsers()
	mux, fs, _, es := newTestMux(syndic)
	token, csrf := loginAs(t, fs, syndic)

	rr := doForm(mux, "POST", "/elections", url.Values{
		"csrf": {csrf}, "title": {"2026 Election"}, "description": {"Choose the next syndic"},
		"transition_days": {"10"},
	}, &http.Cookie{Name: web.SessionCookieName, Value: token}, false)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body: %s", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "/elections/1" {
		t.Errorf("Location = %q, want /elections/1", loc)
	}
	got, err := es.GetElection(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetElection: %v", err)
	}
	if got.TransitionDays != 10 {
		t.Errorf("TransitionDays = %d, want 10", got.TransitionDays)
	}
}

func TestCreateElectionRejectsBadTransitionDays(t *testing.T) {
	syndic, _, _, _ := testUsers()
	mux, fs, _, _ := newTestMux(syndic)
	token, csrf := loginAs(t, fs, syndic)

	rr := doForm(mux, "POST", "/elections", url.Values{
		"csrf": {csrf}, "title": {"2026 Election"}, "transition_days": {"abc"},
	}, &http.Cookie{Name: web.SessionCookieName, Value: token}, false)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 re-render", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "whole number of days") {
		t.Error("bad transition days should re-render the form with a validation error")
	}
}

func TestListPageSettlesExpiredTransition(t *testing.T) {
	syndic, owner1, _, _ := testUsers()
	mux, fs, _, es := newTestMux(syndic, owner1)
	ends := time.Now().Add(-time.Hour)
	e := &model.Election{
		Title:            "2026 Election",
		Status:           model.ElectionStatusTransition,
		CreatedBy:        syndic.ID,
		TransitionEndsAt: &ends,
	}
	w := owner1.ID
	e.WinnerID = &w
	e.WinnerName = owner1.Name
	if err := es.CreateElection(context.Background(), e); err != nil {
		t.Fatalf("seed election: %v", err)
	}

	token, _ := loginAs(t, fs, syndic)
	rr := doGet(mux, "/elections", &http.Cookie{Name: web.SessionCookieName, Value: token})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	after, _ := es.GetElection(context.Background(), e.ID)
	if after.Status != model.ElectionStatusClosed {
		t.Errorf("Status = %q, want closed after lazy settle", after.Status)
	}
	if owner1.Role != model.RoleSyndic {
		t.Errorf("winner role = %q, want syndic after lazy settle", owner1.Role)
	}
	if syndic.Role != model.RoleOwner {
		t.Errorf("previous syndic role = %q, want owner after lazy settle", syndic.Role)
	}
}

func TestCreateElectionRejectsBadCSRF(t *testing.T) {
	syndic, _, _, _ := testUsers()
	mux, fs, _, _ := newTestMux(syndic)
	token, _ := loginAs(t, fs, syndic)

	rr := doForm(mux, "POST", "/elections", url.Values{
		"csrf": {"bogus"}, "title": {"2026 Election"},
	}, &http.Cookie{Name: web.SessionCookieName, Value: token}, false)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 re-render", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "session expired") {
		t.Error("bad csrf should re-render the form with a session error")
	}
}

func TestDetailPageRendersCandidates(t *testing.T) {
	syndic, owner1, _, _ := testUsers()
	mux, fs, _, es := newTestMux(syndic, owner1)
	e := &model.Election{Title: "2026 Election", Status: model.ElectionStatusNomination, CreatedBy: syndic.ID}
	if err := es.CreateElection(context.Background(), e); err != nil {
		t.Fatalf("seed election: %v", err)
	}
	if err := es.CreateCandidate(context.Background(), &model.Candidate{ElectionID: e.ID, UserID: owner1.ID, Statement: "Vote for me"}); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	token, _ := loginAs(t, fs, syndic)

	rr := doGet(mux, fmt.Sprintf("/elections/%d", e.ID), &http.Cookie{Name: web.SessionCookieName, Value: token})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "2026 Election") || !strings.Contains(body, owner1.Name) {
		t.Error("detail page should show the election title and candidates")
	}
	if !strings.Contains(body, "Open voting") {
		t.Error("syndic should see the open-voting action during nominations")
	}
}

func TestVoteHTMXRedirects(t *testing.T) {
	syndic, owner1, owner2, _ := testUsers()
	mux, fs, _, es := newTestMux(syndic, owner1, owner2)
	e := &model.Election{Title: "2026 Election", Status: model.ElectionStatusVoting, CreatedBy: syndic.ID}
	if err := es.CreateElection(context.Background(), e); err != nil {
		t.Fatalf("seed election: %v", err)
	}
	if err := es.CreateCandidate(context.Background(), &model.Candidate{ElectionID: e.ID, UserID: owner1.ID}); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}

	token, csrf := loginAs(t, fs, owner2)
	rr := doForm(mux, "POST", fmt.Sprintf("/elections/%d/vote", e.ID), url.Values{
		"csrf": {csrf}, "candidate_id": {"1"},
	}, &http.Cookie{Name: web.SessionCookieName, Value: token}, true)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for htmx", rr.Code)
	}
	if loc := rr.Header().Get("HX-Redirect"); loc != fmt.Sprintf("/elections/%d", e.ID) {
		t.Errorf("HX-Redirect = %q, want /elections/%d", loc, e.ID)
	}
	voted, err := es.HasVoted(context.Background(), e.ID, owner2.ID)
	if err != nil || !voted {
		t.Errorf("vote should be recorded (voted=%v, err=%v)", voted, err)
	}
}

func TestVoteRejectsWrongCSRF(t *testing.T) {
	syndic, owner1, owner2, _ := testUsers()
	mux, fs, _, es := newTestMux(syndic, owner1, owner2)
	e := &model.Election{Title: "2026 Election", Status: model.ElectionStatusVoting, CreatedBy: syndic.ID}
	if err := es.CreateElection(context.Background(), e); err != nil {
		t.Fatalf("seed election: %v", err)
	}
	if err := es.CreateCandidate(context.Background(), &model.Candidate{ElectionID: e.ID, UserID: owner1.ID}); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}

	token, _ := loginAs(t, fs, owner2)
	rr := doForm(mux, "POST", fmt.Sprintf("/elections/%d/vote", e.ID), url.Values{
		"csrf": {"bogus"}, "candidate_id": {"1"},
	}, &http.Cookie{Name: web.SessionCookieName, Value: token}, false)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 re-render", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "session expired") {
		t.Error("bad csrf should re-render the detail page with a session error")
	}
}

func TestTenantCannotVote(t *testing.T) {
	syndic, owner1, _, tenant := testUsers()
	mux, fs, _, es := newTestMux(syndic, owner1, tenant)
	e := &model.Election{Title: "2026 Election", Status: model.ElectionStatusVoting, CreatedBy: syndic.ID}
	if err := es.CreateElection(context.Background(), e); err != nil {
		t.Fatalf("seed election: %v", err)
	}
	if err := es.CreateCandidate(context.Background(), &model.Candidate{ElectionID: e.ID, UserID: owner1.ID}); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}

	token, csrf := loginAs(t, fs, tenant)
	rr := doForm(mux, "POST", fmt.Sprintf("/elections/%d/vote", e.ID), url.Values{
		"csrf": {csrf}, "candidate_id": {"1"},
	}, &http.Cookie{Name: web.SessionCookieName, Value: token}, false)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 re-render", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Only owners can vote") {
		t.Error("tenant voting should render the owners-only message")
	}
}
