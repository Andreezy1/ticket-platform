package ratelimit_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/ratelimit"
)

func TestLoginEmailKey_DifferentEmailsDifferentKeys(t *testing.T) {
	k1 := ratelimit.LoginEmailKey("alice@example.com")
	k2 := ratelimit.LoginEmailKey("bob@example.com")

	require.NotEqual(t, k1, k2)
}

func TestLoginEmailKey_IsHashedNotRaw(t *testing.T) {
	key := ratelimit.LoginEmailKey("alice@example.com")

	require.True(t, strings.HasPrefix(key, "rl:login:email:"))
	require.NotContains(t, key, "alice",
		"raw email must not appear in the key — it must be hashed")

	hexPart := strings.TrimPrefix(key, "rl:login:email:")
	require.Len(t, hexPart, 64, "sha256 hex digest must be 64 chars")
}

func TestLoginIPKey_NormalizesIPv6Variants(t *testing.T) {
	variants := []string{
		"2001:0db8:0000:0000:0000:0000:0000:0001",
		"2001:db8::1",
		"2001:DB8::1",
	}

	var keys []string
	for _, v := range variants {
		key, ok := ratelimit.LoginIPKey(v)
		require.True(t, ok, "variant %q must be a valid IP", v)
		keys = append(keys, key)
	}

	for i := 1; i < len(keys); i++ {
		require.Equal(t, keys[0], keys[i],
			"IPv6 variant %q must normalize to the same key as %q", variants[i], variants[0])
	}
}

func TestLoginIPKey_DifferentIPsDifferentKeys(t *testing.T) {
	k1, ok1 := ratelimit.LoginIPKey("1.2.3.4")
	k2, ok2 := ratelimit.LoginIPKey("5.6.7.8")

	require.True(t, ok1)
	require.True(t, ok2)
	require.NotEqual(t, k1, k2)
}

func TestLoginIPKey_InvalidInputReturnsFalse(t *testing.T) {
	cases := []string{
		"",
		"not-an-ip",
		"rl:login:ip:9.9.9.9", // попытка коллизии с самим форматом ключа
		"01.02.03.04",         // ведущие нули — net.ParseIP отвергает
	}

	for _, c := range cases {
		key, ok := ratelimit.LoginIPKey(c)
		require.False(t, ok, "input %q must be rejected as invalid", c)
		require.Empty(t, key, "invalid input must not produce a key")
	}
}

func TestLoginIPKey_DifferentPrefixFromEmailKey(t *testing.T) {
	ipKey, ok := ratelimit.LoginIPKey("1.2.3.4")
	require.True(t, ok)
	emailKey := ratelimit.LoginEmailKey("test@example.com")

	require.True(t, strings.HasPrefix(ipKey, "rl:login:ip:"))
	require.True(t, strings.HasPrefix(emailKey, "rl:login:email:"))
	require.NotEqual(t,
		strings.TrimPrefix(ipKey, "rl:login:ip:"),
		strings.TrimPrefix(emailKey, "rl:login:email:"),
	)
}
