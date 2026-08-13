package ratelimit

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"
)

const (
	loginIPPrefix    = "rl:login:ip:"
	loginEmailPrefix = "rl:login:email:"
)

func LoginIPKey(ip string) (key string, ok bool) {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return "", false
	}
	return loginIPPrefix + parsed.String(), true
}

func LoginEmailKey(email string) string {
	return loginEmailPrefix + hashEmail(email)
}

func hashEmail(email string) string {
	sum := sha256.Sum256([]byte(email))
	return hex.EncodeToString(sum[:])
}
