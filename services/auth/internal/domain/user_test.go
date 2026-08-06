package domain

import (
	"errors"
	"strings"
	"testing"
)

func validUser() UserParams {
	return UserParams{
		Email:    "CaspeR@mail.ru",
		Password: "Dsdf242dsaad2dsa",
	}
}

func TestUserParams_Validate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(u *UserParams)
		wantErr   error
		wantEmail string
	}{
		{
			name:      "valid user passes without changes",
			mutate:    func(u *UserParams) {},
			wantErr:   nil,
			wantEmail: "casper@mail.ru",
		},
		{
			name:    "empty email returns validation error",
			mutate:  func(u *UserParams) { u.Email = "" },
			wantErr: ErrValidate,
		},
		{
			name:    "whitespace-only email returns validation error",
			mutate:  func(u *UserParams) { u.Email = "        " },
			wantErr: ErrValidate,
		},
		{
			name:      "email is trimmed",
			mutate:    func(u *UserParams) { u.Email = "  klodsa@mail.ru   " },
			wantErr:   nil,
			wantEmail: "klodsa@mail.ru",
		},
		{
			name:      "email normalized to lowercase",
			mutate:    func(u *UserParams) { u.Email = "KLODSA@mail.ru" },
			wantErr:   nil,
			wantEmail: "klodsa@mail.ru",
		},
		{
			name:    "email without @ returns validation error",
			mutate:  func(u *UserParams) { u.Email = "klodsamail.ru" },
			wantErr: ErrValidate,
		},
		{
			name:    "email containing a space returns validation error",
			mutate:  func(u *UserParams) { u.Email = "Jhon a@b.ru" },
			wantErr: ErrValidate,
		},
		{
			name:    "email with display name is rejected",
			mutate:  func(u *UserParams) { u.Email = "Jhon <a@b.ru>" },
			wantErr: ErrValidate,
		},
		{
			name:    "empty password returns validation error",
			mutate:  func(u *UserParams) { u.Password = "" },
			wantErr: ErrValidate,
		},
		{
			name:    "9-character password returns validation error",
			mutate:  func(u *UserParams) { u.Password = "123456789" },
			wantErr: ErrValidate,
		},
		{
			name:    "10-character password is accepted",
			mutate:  func(u *UserParams) { u.Password = "1234567890" },
			wantErr: nil,
		},
		{
			name:    "72 byte password is accepted",
			mutate:  func(u *UserParams) { u.Password = strings.Repeat("a", 72) },
			wantErr: nil,
		},
		{
			name:    "73 byte password returns validation error",
			mutate:  func(u *UserParams) { u.Password = strings.Repeat("a", 73) },
			wantErr: ErrValidate,
		},
		{
			name:    "20-byte cyrillic password is accepted",
			mutate:  func(u *UserParams) { u.Password = strings.Repeat("Б", 10) },
			wantErr: nil,
		},
		{
			name:    "80-byte cyrillic password returns error",
			mutate:  func(u *UserParams) { u.Password = strings.Repeat("Б", 40) },
			wantErr: ErrValidate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := validUser()
			tt.mutate(&user)

			err := user.Validate()
			if tt.wantEmail != "" {
				if user.Email != tt.wantEmail {
					t.Errorf("email not normalized to lowercase: got %q, want %q", user.Email, tt.wantEmail)
				}
			}
			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected validation error, got nil")
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("error %v is not wrapped with ErrValidate", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}
