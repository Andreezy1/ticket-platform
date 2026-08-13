package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
	"github.com/Andreezy1/ticket-platform/services/auth/internal/ratelimit"
	"github.com/stretchr/testify/require"
)

type fakeLimiter struct {
	allow        bool
	calledKey    string
	calledLimit  int
	calledWindow time.Duration
	calledN      int
}

func (f *fakeLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) bool {
	f.calledKey = key
	f.calledLimit = limit
	f.calledWindow = window
	f.calledN++
	return f.allow
}

func TestIPRateLimit_Allowed_CallsNext(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	limiter := &fakeLimiter{allow: true}
	mw := NewIPRateLimit(limiter, 5, time.Minute)

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	require.True(t, nextCalled, "next must be called when Allow returns true")
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestIPRateLimit_Rejected_ReturnsIdenticalBodyAsServicePath(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	limiter := &fakeLimiter{allow: false}
	mw := NewIPRateLimit(limiter, 5, time.Minute)

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	require.False(t, nextCalled, "next must NOT be called on rejection — this is the critical assertion")
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	// Тело должно совпадать с тем, что даст writeServiceError на
	// сервисном пути для той же ошибки — сравниваем напрямую с ним,
	// а не с захардкоженной строкой, иначе тест не поймает рассинхрон,
	// если кто-то поменяет формат в одном месте и забудет про другое.
	wantRec := httptest.NewRecorder()
	writeServiceError(wantRec, domain.ErrTooManyRequests)

	require.Equal(t, wantRec.Body.String(), rec.Body.String())
	require.Equal(t, wantRec.Header().Get("Retry-After"), rec.Header().Get("Retry-After"))
}

func TestIPRateLimit_BuildsKeyFromRemoteAddr(t *testing.T) {
	limiter := &fakeLimiter{allow: true}
	mw := NewIPRateLimit(limiter, 5, time.Minute)

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec, req)

	wantKey, ok := ratelimit.LoginIPKey("1.2.3.4")
	require.True(t, ok)
	require.Equal(t, wantKey, limiter.calledKey)
}

func TestIPRateLimit_IPv6_DoesNotPanic(t *testing.T) {
	limiter := &fakeLimiter{allow: true}
	mw := NewIPRateLimit(limiter, 5, time.Minute)

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "[2001:db8::1]:5678"
	rec := httptest.NewRecorder()

	require.NotPanics(t, func() {
		mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec, req)
	})
	require.Equal(t, 1, limiter.calledN, "limiter must have been called with a parsed key")
}

func TestIPRateLimit_RemoteAddrWithoutPort_FailsOpen(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	limiter := &fakeLimiter{allow: false} // даже если бы дошло до Allow, вернуло бы false
	mw := NewIPRateLimit(limiter, 5, time.Minute)

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "1.2.3.4" // без порта — SplitHostPort вернёт ошибку
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	require.True(t, nextCalled, "malformed RemoteAddr must fail open, not block")
	require.Equal(t, 0, limiter.calledN, "limiter must not be called at all when host can't be parsed")
}

func TestIPRateLimit_UsesConfiguredLimitAndWindow_NotHardcoded(t *testing.T) {
	limiter := &fakeLimiter{allow: true}
	// Специально нестандартные значения — если middleware где-то
	// захардкодил свои цифры вместо передачи полученных параметров,
	// сравнение поймает это только опосредованно (Allow не различает
	// limit/window в возврате), поэтому здесь просто фиксируем сам факт
	// вызова с ненулевым, не дефолтным конфигом — как smoke-проверку
	// на регрессию сигнатуры.
	customLimit := 17
	customWindow := 42 * time.Second

	mw := NewIPRateLimit(limiter, customLimit, customWindow)

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec, req)
	require.Equal(t, customLimit, limiter.calledLimit)
	require.Equal(t, customWindow, limiter.calledWindow)
}
