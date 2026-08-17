package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
)

// ---------------------------------------------------------------------
// strictRecorder — like httptest.ResponseRecorder, but actually enforces
// the real net/http contract: once WriteHeader is called, headers are
// frozen. httptest.ResponseRecorder doesn't simulate this, so a mutant
// that swaps Header().Set and WriteteHeader() order would slip through
// a plain ResponseRecorder-based test.
// ---------------------------------------------------------------------

type strictRecorder struct {
	header        http.Header
	code          int
	wroteHeader   bool
	body          bytes.Buffer
	headerAtWrite http.Header // snapshot of headers at the moment WriteHeader was called
}

func newStrictRecorder() *strictRecorder {
	return &strictRecorder{header: http.Header{}}
}

func (r *strictRecorder) Header() http.Header { return r.header }

func (r *strictRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.code = code
	r.headerAtWrite = r.header.Clone()
}

func (r *strictRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	return r.body.Write(b)
}

// ---------------------------------------------------------------------
// Этап A. statusFromErr — таблица без единого мока
// ---------------------------------------------------------------------

func TestStatusFromErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"ErrNotFound", domain.ErrNotFound, http.StatusNotFound},
		{"ErrConflict", domain.ErrConflict, http.StatusConflict},
		{"ErrValidate", domain.ErrValidate, http.StatusBadRequest},
		{"ErrInvalidCredentials", domain.ErrInvalidCredentials, http.StatusUnauthorized},
		{"ErrAccountDisabled", domain.ErrAccountDisabled, http.StatusForbidden},
		{"ErrInvalidRefreshToken", domain.ErrInvalidRefreshToken, http.StatusUnauthorized},
		{"ErrTooManyRequests", domain.ErrTooManyRequests, http.StatusTooManyRequests},
		{
			"wrapped ErrValidate via %w",
			fmt.Errorf("%w: email already registered", domain.ErrValidate),
			http.StatusBadRequest,
		},
		{
			"unknown error",
			errors.New("something exploded"),
			http.StatusInternalServerError,
		},
		{
			// Решение: nil не должен попадать в этот код в проде (writeServiceError
			// вызывается только когда err != nil), но функция обязана вести себя
			// предсказуемо, если это когда-нибудь нарушится: любой errors.Is(nil, X)
			// возвращает false, значит nil должен уйти в default -> 500.
			"nil error falls through to default (500)",
			nil,
			http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := statusFromErr(tt.err)
			if got != tt.want {
				t.Errorf("statusFromErr(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

// Отдельно проверяем, что маппинг действительно идёт через errors.Is,
// а не через прямое сравнение указателей: двойная обёртка тоже должна
// резолвиться правильно.
func TestStatusFromErr_DoubleWrapped(t *testing.T) {
	err := fmt.Errorf("repo: %w", fmt.Errorf("service: %w", domain.ErrConflict))
	got := statusFromErr(err)
	if got != http.StatusConflict {
		t.Errorf("double-wrapped ErrConflict: got %d, want %d", got, http.StatusConflict)
	}
}

// ---------------------------------------------------------------------
// Этап B. writeServiceError — защита от утечки
// ---------------------------------------------------------------------

func TestWriteServiceError_ClientError_IncludesMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	err := fmt.Errorf("%w: email is required", domain.ErrValidate)

	writeServiceError(rec, err)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var payload map[string]string
	if decErr := json.Unmarshal(rec.Body.Bytes(), &payload); decErr != nil {
		t.Fatalf("failed to decode body: %v", decErr)
	}
	if payload["error"] != err.Error() {
		t.Errorf("body error = %q, want %q", payload["error"], err.Error())
	}
}

func TestWriteServiceError_ServerError_IsGeneric(t *testing.T) {
	rec := httptest.NewRecorder()
	err := errors.New("boom: something internal failed")

	writeServiceError(rec, err)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	got := strings.TrimSpace(rec.Body.String())
	want := `{"error":"internal server error"}`
	if got != want {
		t.Errorf("body = %q, want exactly %q", got, want)
	}
}

// Регрессия шага 2: детали БД (имя constraint/индекса) не должны
// просачиваться в ответ клиенту.
func TestWriteServiceError_DoesNotLeakDBDetails(t *testing.T) {
	rec := httptest.NewRecorder()

	dbDetail := `duplicate key value violates unique constraint "users_email_lower_idx"`
	err := fmt.Errorf("repository unavailable: %w", errors.New(dbDetail))

	writeServiceError(rec, err)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (unmapped errors must default to 500)", rec.Code, http.StatusInternalServerError)
	}

	body := rec.Body.String()
	if strings.Contains(body, "users_email_lower_idx") {
		t.Errorf("response body leaked DB index name: %s", body)
	}
	if strings.Contains(body, "duplicate key value") {
		t.Errorf("response body leaked DB error text: %s", body)
	}

	want := `{"error":"internal server error"}` + "\n"
	if body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

// ---------------------------------------------------------------------
// Этап C. writeJSON — порядок вызовов
// ---------------------------------------------------------------------

func TestWriteJSON_OrdinaryBody(t *testing.T) {
	rec := newStrictRecorder()

	writeJSON(rec, http.StatusCreated, map[string]string{"id": "123"})

	if rec.code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.code, http.StatusCreated)
	}
	// Проверяем заголовок ИМЕННО в момент вызова WriteHeader — это и ловит
	// мутацию, переставляющую Header().Set после WriteHeader().
	if ct := rec.headerAtWrite.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type at WriteHeader time = %q, want %q", ct, "application/json")
	}

	var got map[string]string
	if err := json.Unmarshal(rec.body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if got["id"] != "123" {
		t.Errorf("body id = %q, want %q", got["id"], "123")
	}
}

func TestWriteJSON_NilBody(t *testing.T) {
	rec := newStrictRecorder()

	writeJSON(rec, http.StatusNoContent, nil)

	if rec.code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.code, http.StatusNoContent)
	}
	if ct := rec.headerAtWrite.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type at WriteHeader time = %q, want %q", ct, "application/json")
	}
	if rec.body.Len() != 0 {
		t.Errorf("body should be empty for nil, got %q", rec.body.String())
	}
}

// ---------------------------------------------------------------------
// Этап D. decodeJSON
// ---------------------------------------------------------------------

type decodeTarget struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func newRequestWithBody(t *testing.T, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestDecodeJSON_ValidBody(t *testing.T) {
	rec := httptest.NewRecorder()
	req := newRequestWithBody(t, `{"name":"Alice","age":30}`)

	var target decodeTarget
	if err := decodeJSON(rec, req, &target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Name != "Alice" || target.Age != 30 {
		t.Errorf("target = %+v, want {Alice 30}", target)
	}
}

func TestDecodeJSON_UnknownField(t *testing.T) {
	rec := httptest.NewRecorder()
	req := newRequestWithBody(t, `{"name":"Alice","surprise":"field"}`)

	var target decodeTarget
	if err := decodeJSON(rec, req, &target); err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
}

func TestDecodeJSON_MalformedJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := newRequestWithBody(t, `{"name": "Alice", "age":`) // truncated

	var target decodeTarget
	if err := decodeJSON(rec, req, &target); err == nil {
		t.Fatal("expected error for malformed json, got nil")
	}
}

func TestDecodeJSON_BodyTooLarge(t *testing.T) {
	rec := httptest.NewRecorder()

	// Больше 1 МБ: валидный JSON с длинной строкой, чтобы точно превысить лимит.
	huge := strings.Repeat("a", (1<<20)+1024)
	body := fmt.Sprintf(`{"name":%q,"age":1}`, huge)
	req := newRequestWithBody(t, body)

	var target decodeTarget
	err := decodeJSON(rec, req, &target)
	if err == nil {
		t.Fatal("expected error for body > 1MB, got nil (MaxBytesReader missing?)")
	}
}
