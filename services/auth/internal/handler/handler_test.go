package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
	"github.com/google/uuid"
)

type fakeUserService struct {
	// Create
	createUser   *domain.User
	createErr    error
	calledCreate *domain.UserParams
	createCalls  int

	// Login
	loginTokens *domain.Tokens
	loginErr    error
	calledLogin *domain.UserParams
	loginCalls  int

	// Refresh
	refreshTokens *domain.Tokens
	refreshErr    error
	calledRefresh string
	refreshCalls  int

	// Logout
	logoutErr    error
	calledLogout string
	logoutCalls  int
}

func (f *fakeUserService) Create(_ context.Context, params *domain.UserParams) (*domain.User, error) {
	f.calledCreate = params
	f.createCalls++
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.createUser, nil
}

func (f *fakeUserService) Login(_ context.Context, params *domain.UserParams) (*domain.Tokens, error) {
	f.calledLogin = params
	f.loginCalls++
	if f.loginErr != nil {
		return nil, f.loginErr
	}
	return f.loginTokens, nil
}

func (f *fakeUserService) Refresh(_ context.Context, plain string) (*domain.Tokens, error) {
	f.calledRefresh = plain
	f.refreshCalls++
	if f.refreshErr != nil {
		return nil, f.refreshErr
	}
	return f.refreshTokens, nil
}

func (f *fakeUserService) Logout(_ context.Context, plain string) error {
	f.calledLogout = plain
	f.logoutCalls++
	return f.logoutErr
}

// ---------------------------------------------------------------------
// fakeTokenJWKS
// ---------------------------------------------------------------------

type fakeTokenJWKS struct {
	keys []domain.JWK
}

func (f *fakeTokenJWKS) JWKS() []domain.JWK {
	return f.keys
}

// ---------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------

func newJSONRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// ---------------------------------------------------------------------
// create
// ---------------------------------------------------------------------

func TestHandler_Create_Success(t *testing.T) {
	svc := &fakeUserService{
		createUser: &domain.User{
			Id:        uuid.New(),
			Email:     "user@example.com",
			CreatedAt: time.Now(),
		},
	}
	h := NewHandler(svc, &fakeTokenJWKS{}, 15*time.Minute)

	req := newJSONRequest(t, http.MethodPost, "/register", `{"email":"user@example.com","password":"supersecretpw"}`)
	rec := httptest.NewRecorder()

	h.create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if rec.Body.Len() == 0 {
		t.Fatal("expected non-empty body on 201")
	}

	var resp createUserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if resp.Email != "user@example.com" {
		t.Errorf("resp.Email = %q, want %q", resp.Email, "user@example.com")
	}

	if svc.calledCreate == nil || svc.calledCreate.Email != "user@example.com" {
		t.Errorf("service.Create called with %+v, want email %q", svc.calledCreate, "user@example.com")
	}
}

func TestHandler_Create_DecodeError(t *testing.T) {
	svc := &fakeUserService{}
	h := NewHandler(svc, &fakeTokenJWKS{}, 15*time.Minute)

	req := newJSONRequest(t, http.MethodPost, "/register", `{"email": "broken"`) // truncated json
	rec := httptest.NewRecorder()

	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("expected non-empty body describing the decode error, got empty body")
	}
	if svc.calledCreate != nil {
		t.Errorf("service.Create should not be called on decode error, but was called with %+v", svc.calledCreate)
	}
	if svc.createCalls != 0 {
		t.Errorf("service.Create call count = %d, want 0", svc.createCalls)
	}
}

// ---------------------------------------------------------------------
// login
// ---------------------------------------------------------------------

func TestHandler_Login_Success(t *testing.T) {
	svc := &fakeUserService{
		loginTokens: &domain.Tokens{
			AccessToken:  "access-123",
			RefreshToken: "refresh-123",
		},
	}
	ttl := 15 * time.Minute
	h := NewHandler(svc, &fakeTokenJWKS{}, ttl)

	req := newJSONRequest(t, http.MethodPost, "/login", `{"email":"user@example.com","password":"supersecretpw"}`)
	rec := httptest.NewRecorder()

	h.login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp tokensResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}

	if resp.TokenType != "Bearer" {
		t.Errorf("token_type = %q, want %q", resp.TokenType, "Bearer")
	}
	if resp.ExpiresIn != 900 {
		t.Errorf("expires_in = %d, want %d (ttl=%s)", resp.ExpiresIn, 900, ttl)
	}
	if resp.AccessToken != "access-123" || resp.RefreshToken != "refresh-123" {
		t.Errorf("tokens = %+v, want access=access-123 refresh=refresh-123", resp)
	}

	if svc.calledLogin == nil || svc.calledLogin.Email != "user@example.com" {
		t.Errorf("service.Login called with %+v, want email %q", svc.calledLogin, "user@example.com")
	}
}

func TestHandler_Login_ServiceError(t *testing.T) {
	svc := &fakeUserService{loginErr: domain.ErrInvalidCredentials}
	h := NewHandler(svc, &fakeTokenJWKS{}, 15*time.Minute)

	req := newJSONRequest(t, http.MethodPost, "/login", `{"email":"user@example.com","password":"wrong"}`)
	rec := httptest.NewRecorder()

	h.login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandler_Login_DecodeError(t *testing.T) {
	svc := &fakeUserService{
		loginTokens: &domain.Tokens{AccessToken: "should-not-appear", RefreshToken: "should-not-appear"},
	}
	h := NewHandler(svc, &fakeTokenJWKS{}, 15*time.Minute)

	req := newJSONRequest(t, http.MethodPost, "/login", `{"email": "user@example.com", "password":`) // truncated
	rec := httptest.NewRecorder()

	h.login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	if svc.calledLogin != nil {
		t.Errorf("service.Login must not be called after a decode error, but was called with %+v", svc.calledLogin)
	}
	if svc.loginCalls != 0 {
		t.Errorf("service.Login call count = %d, want 0", svc.loginCalls)
	}

	var payload map[string]any
	dec := json.NewDecoder(bytes.NewReader(rec.Body.Bytes()))
	if err := dec.Decode(&payload); err != nil {
		t.Fatalf("failed to decode body as single JSON object: %v, body=%q", err, rec.Body.String())
	}
	if dec.More() {
		t.Fatalf("body contains more than one JSON value (second response leaked through): %q", rec.Body.String())
	}
	if _, ok := payload["error"]; !ok {
		t.Errorf(`body missing "error" field, got: %q`, rec.Body.String())
	}
	if _, ok := payload["access_token"]; ok {
		t.Errorf("body unexpectedly contains access_token from a second, unguarded response: %q", rec.Body.String())
	}
}

// ---------------------------------------------------------------------
// refresh
// ---------------------------------------------------------------------

func TestHandler_Refresh_Success(t *testing.T) {
	svc := &fakeUserService{
		refreshTokens: &domain.Tokens{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
		},
	}
	h := NewHandler(svc, &fakeTokenJWKS{}, 15*time.Minute)

	req := newJSONRequest(t, http.MethodPost, "/refresh", `{"refresh_token":"old-refresh"}`)
	rec := httptest.NewRecorder()

	h.refresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp tokensResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if resp.RefreshToken != "new-refresh" {
		t.Errorf("refresh_token = %q, want %q (must be the NEW token, not the old one)", resp.RefreshToken, "new-refresh")
	}
	if resp.AccessToken != "new-access" {
		t.Errorf("access_token = %q, want %q", resp.AccessToken, "new-access")
	}

	if svc.calledRefresh != "old-refresh" {
		t.Errorf("service.Refresh called with %q, want %q", svc.calledRefresh, "old-refresh")
	}
}

func TestHandler_Refresh_ServiceError(t *testing.T) {
	svc := &fakeUserService{refreshErr: domain.ErrInvalidRefreshToken}
	h := NewHandler(svc, &fakeTokenJWKS{}, 15*time.Minute)

	req := newJSONRequest(t, http.MethodPost, "/refresh", `{"refresh_token":"bad-token"}`)
	rec := httptest.NewRecorder()

	h.refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandler_Refresh_DecodeError(t *testing.T) {
	svc := &fakeUserService{}
	h := NewHandler(svc, &fakeTokenJWKS{}, 15*time.Minute)

	req := newJSONRequest(t, http.MethodPost, "/refresh", `not json at all`)
	rec := httptest.NewRecorder()

	h.refresh(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if svc.refreshCalls != 0 {
		t.Errorf("service.Refresh call count = %d, want 0 (called=%q)", svc.refreshCalls, svc.calledRefresh)
	}
}

func TestHandler_Logout_Success_NoContentAndEmptyBody(t *testing.T) {
	svc := &fakeUserService{}
	h := NewHandler(svc, &fakeTokenJWKS{}, 15*time.Minute)

	req := newJSONRequest(t, http.MethodPost, "/logout", `{"refresh_token":"some-refresh"}`)
	rec := httptest.NewRecorder()

	h.logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body length = %d, want 0, got %q", rec.Body.Len(), rec.Body.String())
	}
	if svc.calledLogout != "some-refresh" {
		t.Errorf("service.Logout called with %q, want %q", svc.calledLogout, "some-refresh")
	}
}

func TestHandler_Logout_ServiceError(t *testing.T) {
	svc := &fakeUserService{logoutErr: domain.ErrInvalidRefreshToken}
	h := NewHandler(svc, &fakeTokenJWKS{}, 15*time.Minute)

	req := newJSONRequest(t, http.MethodPost, "/logout", `{"refresh_token":"bad"}`)
	rec := httptest.NewRecorder()

	h.logout(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected non-empty error body on failure, got empty body")
	}
}

func TestHandler_Logout_DecodeError(t *testing.T) {
	svc := &fakeUserService{}
	h := NewHandler(svc, &fakeTokenJWKS{}, 15*time.Minute)

	req := newJSONRequest(t, http.MethodPost, "/logout", `{"refresh_token": }`) // malformed
	rec := httptest.NewRecorder()

	h.logout(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if svc.logoutCalls != 0 {
		t.Errorf("service.Logout call count = %d, want 0 (called with %q)", svc.logoutCalls, svc.calledLogout)
	}
}

func TestHandler_JWKS_Success(t *testing.T) {
	svc := &fakeUserService{}
	jwks := &fakeTokenJWKS{
		keys: []domain.JWK{
			{Kty: "OKP", Crv: "Ed25519", X: "base64x", Kid: "kid-1", Use: "sig", Alg: "EdDSA"},
		},
	}
	h := NewHandler(svc, jwks, 15*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	rec := newStrictRecorder()

	h.jwks(rec, req)

	if rec.code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.code, http.StatusOK)
	}

	if cc := rec.headerAtWrite.Get("Cache-Control"); cc != "public, max-age=3600" {
		t.Errorf("Cache-Control at WriteHeader time = %q, want %q", cc, "public, max-age=3600")
	}
	if ct := rec.headerAtWrite.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type at WriteHeader time = %q, want %q", ct, "application/json")
	}

	var resp map[string][]jWKSResponse
	if err := json.Unmarshal(rec.body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	keys, ok := resp["keys"]
	if !ok {
		t.Fatal(`response missing "keys" field`)
	}
	if len(keys) != 1 || keys[0].Kid != "kid-1" {
		t.Errorf("keys = %+v, want one key with kid=kid-1", keys)
	}
}

func TestHandler_JWKS_EmptyKeys(t *testing.T) {
	svc := &fakeUserService{}
	jwks := &fakeTokenJWKS{keys: []domain.JWK{}}
	h := NewHandler(svc, jwks, 15*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	rec := httptest.NewRecorder()

	h.jwks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	got := strings.TrimSpace(rec.Body.String())
	want := `{"keys":[]}`
	if got != want {
		t.Errorf("body = %q, want %q (empty keys must serialize as [], not null)", got, want)
	}
}
