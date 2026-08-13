package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/ratelimit"
)

func newTestLimiter(t *testing.T) (*ratelimit.Limiter, *miniredis.Miniredis, *redis.Client) {
	t.Helper()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	limiter := ratelimit.NewLimiter(rdb, time.Second)
	return limiter, mr, rdb
}

func TestAllow_NCallsWithinLimitAllSucceed(t *testing.T) {
	limiter, _, _ := newTestLimiter(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		require.True(t, limiter.Allow(ctx, "k1", 3, 10*time.Second),
			"call %d out of 3 should be allowed", i+1)
	}
}

func TestAllow_NPlusOneCallRejected(t *testing.T) {
	limiter, _, _ := newTestLimiter(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		require.True(t, limiter.Allow(ctx, "k1", 3, 10*time.Second))
	}

	require.False(t, limiter.Allow(ctx, "k1", 3, 10*time.Second),
		"4th call over limit 3 must be rejected")
}

func TestAllow_ZCardAfterRejection_EqualsLimit(t *testing.T) {
	limiter, _, rdb := newTestLimiter(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		require.True(t, limiter.Allow(ctx, "k1", 3, 10*time.Second))
	}
	require.False(t, limiter.Allow(ctx, "k1", 3, 10*time.Second))

	// Вариант А: отклонённый вызов не пишется в ZSET — card остаётся
	// равен лимиту, а не растёт до limit+1.
	card, err := rdb.ZCard(ctx, "k1").Result()
	require.NoError(t, err)
	require.EqualValues(t, 3, card)
}

func TestAllow_EvictsStaleEntriesWhileKeyStillAlive(t *testing.T) {
	limiter, _, rdb := newTestLimiter(t)
	ctx := context.Background()
	window := 10 * time.Second

	// Кладём 3 записи вручную с score далеко за пределами окна
	// (now - 2*window), но ключ при этом жив — TTL мы не трогаем.
	// Если бы тест полагался на истечение ключа (как ошибочно
	// предполагалось раньше через FastForward), этот сценарий
	// его бы не поймал: TTL тут ни при чём, ключ никогда не удалялся.
	staleScore := float64(time.Now().Add(-2 * window).UnixMilli())
	for i, member := range []string{"stale-1", "stale-2", "stale-3"} {
		err := rdb.ZAdd(ctx, "k1", redis.Z{Score: staleScore + float64(i), Member: member}).Err()
		require.NoError(t, err)
	}

	card, err := rdb.ZCard(ctx, "k1").Result()
	require.NoError(t, err)
	require.EqualValues(t, 3, card, "sanity check: stale entries are actually in the set")

	// Лимит 3, но все три записи вне окна — Allow обязан их вычистить
	// через ZREMRANGEBYSCORE и разрешить вызов, увидев count=0 после
	// очистки. Без ZREMRANGEBYSCORE (или если ZCARD считается до
	// очистки) count был бы >= limit, и Allow вернул бы false.
	allowed := limiter.Allow(ctx, "k1", 3, window)
	require.True(t, allowed, "stale entries outside the window must not count toward the limit")

	// После успешного Allow в множестве должна остаться ровно одна
	// запись — новый вызов; три протухшие должны быть удалены.
	card, err = rdb.ZCard(ctx, "k1").Result()
	require.NoError(t, err)
	require.EqualValues(t, 1, card, "stale entries must be evicted, only the new call remains")
}

func TestAllow_AfterKeyExpires_AllowsAgain(t *testing.T) {
	limiter, mr, _ := newTestLimiter(t)
	ctx := context.Background()
	window := 10 * time.Second

	for i := 0; i < 3; i++ {
		require.True(t, limiter.Allow(ctx, "k1", 3, window))
	}
	require.False(t, limiter.Allow(ctx, "k1", 3, window))

	// Важно: FastForward в miniredis двигает только TTL, не команду
	// TIME — внутри скрипта "now" остаётся реальным временем теста.
	// Поэтому этот тест проверяет не работу ZREMRANGEBYSCORE (это
	// покрыто отдельным тестом выше), а куда более простой факт:
	// когда PEXPIRE истекает и ключ целиком удаляется Redis'ом,
	// лимитер должен благополучно начать с чистого листа — не
	// упасть на отсутствующем ключе и не считать это ошибкой.
	mr.FastForward(window + time.Second)

	require.True(t, limiter.Allow(ctx, "k1", 3, window),
		"after key TTL expires, a fresh key must allow calls again")
}

func TestReset_ClearsCounterMidWindow(t *testing.T) {
	limiter, _, rdb := newTestLimiter(t)
	ctx := context.Background()
	window := 10 * time.Second

	for i := 0; i < 3; i++ {
		require.True(t, limiter.Allow(ctx, "k1", 3, window))
	}
	require.False(t, limiter.Allow(ctx, "k1", 3, window), "limit must be exhausted before reset")

	limiter.Reset(ctx, "k1")

	card, err := rdb.Exists(ctx, "k1").Result()
	require.NoError(t, err)
	require.EqualValues(t, 0, card, "key must not exist after reset")

	require.True(t, limiter.Allow(ctx, "k1", 3, window),
		"after reset, calls within the same original window must be allowed again")
}

func TestAllow_DifferentKeysDoNotAffectEachOther(t *testing.T) {
	limiter, _, _ := newTestLimiter(t)
	ctx := context.Background()
	window := 10 * time.Second

	for i := 0; i < 3; i++ {
		require.True(t, limiter.Allow(ctx, "k1", 3, window))
	}
	require.False(t, limiter.Allow(ctx, "k1", 3, window), "k1 must be exhausted")

	// k2 — отдельное ведро, не должно унаследовать исчерпание k1.
	require.True(t, limiter.Allow(ctx, "k2", 3, window),
		"k2 must be unaffected by k1's exhausted limit")
}

func TestAllow_RedisDown_FailsOpen(t *testing.T) {
	limiter, mr, _ := newTestLimiter(t)
	ctx := context.Background()

	mr.Close()

	// Это ключевой тест политики: если Redis недоступен, Allow обязан
	// вернуть true. Если кто-то однажды заменит return true на false
	// в обработке ошибки — этот тест должен упасть первым.
	allowed := limiter.Allow(ctx, "k1", 3, 10*time.Second)
	require.True(t, allowed, "limiter must fail open when redis is unavailable")
}
