package service_test

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/service"
	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	slog.SetDefault(discardLogger)

	h, err := bcrypt.GenerateFromPassword([]byte("test-dummy"), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	service.SetDummyHash(h)
	service.SetBcryptCost(bcrypt.MinCost)

	os.Exit(m.Run())
}
