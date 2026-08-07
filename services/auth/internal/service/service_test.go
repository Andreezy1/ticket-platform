package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
	"github.com/Andreezy1/ticket-platform/services/auth/internal/service"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var timeDB = time.Date(2026, 01, 01, 12, 0, 0, 0, time.UTC)

var (
	testUserID   = uuid.MustParse("f47ac10b-58cc-4372-a567-0e02b2c3d479")
	testTokenID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	replacedByID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
)

type stand struct {
	svc               *service.UserService
	userRepo          *fakeUserRepo
	refreshTokenRepo  *fakeTokenRepo
	tokens            *fakeManager
	refreshTokenTTL   time.Duration
	refreshReuseGrace time.Duration
}

func newStand(t *testing.T, refreshTokenTTL time.Duration, refreshReuseGrace time.Duration) *stand {
	t.Helper()
	s := &stand{
		userRepo:          &fakeUserRepo{},
		refreshTokenRepo:  &fakeTokenRepo{},
		tokens:            &fakeManager{},
		refreshTokenTTL:   refreshTokenTTL,
		refreshReuseGrace: refreshReuseGrace,
	}
	s.svc = service.NewUserService(s.userRepo, s.refreshTokenRepo, s.tokens, s.refreshTokenTTL, s.refreshReuseGrace)
	return s
}

func newValidRefreshToken() *domain.RefreshToken {
	return &domain.RefreshToken{
		ID:                testTokenID,
		UserID:            testUserID,
		TokenHash:         "hashed-token",
		ExpiresAt:         timeDB.Add(24 * time.Hour),
		CreatedAt:         timeDB,
		RevokedAt:         nil,
		ReplacedByTokenID: nil,
	}
}

func TestUserService_Logout(t *testing.T) {
	tests := []struct {
		name          string
		enterPlain    string
		mutate        func(u *stand)
		wantCountCall int
		wantErr       error
		wantArg       string
	}{
		{
			name:       "success",
			enterPlain: "plain",
			mutate: func(u *stand) {
				u.refreshTokenRepo.revokeByHashErr = nil
			},
			wantCountCall: 1,
			wantErr:       nil,
			wantArg:       "hashed:plain",
		},
		{
			name:       "error from repo",
			enterPlain: "plain",
			mutate: func(u *stand) {
				u.refreshTokenRepo.revokeByHashErr = repoErr
			},
			wantCountCall: 1,
			wantErr:       repoErr,
			wantArg:       "hashed:plain",
		},
		{
			name:          "empty plain token",
			enterPlain:    "",
			mutate:        func(u *stand) {},
			wantCountCall: 0,
			wantErr:       domain.ErrValidate,
			wantArg:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStand(t, 0, 0)
			tt.mutate(s)
			err := s.svc.Logout(context.Background(), tt.enterPlain)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Logout() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(s.refreshTokenRepo.revokeByHashArgs) != tt.wantCountCall {
				t.Errorf("Logout() called with %d args, want %d", len(s.refreshTokenRepo.revokeByHashArgs), tt.wantCountCall)
			}
			if tt.wantCountCall == 0 {
				return
			}
			if s.refreshTokenRepo.revokeByHashArgs[0] != tt.wantArg {
				t.Errorf("Logout() called with hash = %v, wantArg %v", s.refreshTokenRepo.revokeByHashArgs[0], tt.wantArg)
			}

		})
	}
}

func TestUserService_Refresh_AfterRotation(t *testing.T) {
	tests := []struct {
		name                 string
		enterPlain           string
		mutate               func(u *stand)
		wantRotateCall       int
		wantRevokeByHashCall int
		wantAccessCall       int
		wantErr              error
	}{
		{
			name:       "success",
			enterPlain: "plain",
			mutate: func(u *stand) {
				u.refreshTokenRepo.rotateUserID = testUserID
				u.userRepo.findByIDUser = &domain.User{
					Id:       testUserID,
					Role:     "user",
					IsActive: true,
				}
				u.refreshTokenRepo.rotateErr = nil
				u.tokens.genPlain = "new-plain"
				u.tokens.genHash = "hashed:new-plain"
				u.tokens.accessToken = "access-token"
			},
			wantRotateCall: 1,
			wantAccessCall: 1,
			wantErr:        nil,
		},
		{
			name:       "user not active",
			enterPlain: "plain",
			mutate: func(u *stand) {
				u.refreshTokenRepo.rotateUserID = testUserID
				u.userRepo.findByIDUser = &domain.User{
					Id:       testUserID,
					Role:     "user",
					IsActive: false,
				}
				u.refreshTokenRepo.rotateErr = nil
				u.tokens.genPlain = "new-plain"
				u.tokens.genHash = "hashed:new-plain"
				u.tokens.accessToken = "access-token"
			},
			wantRotateCall:       1,
			wantRevokeByHashCall: 1,
			wantAccessCall:       0,
			wantErr:              domain.ErrAccountDisabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStand(t, 24*time.Hour, 15*time.Second)
			tt.mutate(s)
			tkn, err := s.svc.Refresh(context.Background(), tt.enterPlain)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Refresh() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(s.refreshTokenRepo.rotateCalls) != tt.wantRotateCall {
				t.Fatalf("Refresh() called with %d args, want %d", len(s.refreshTokenRepo.rotateCalls), tt.wantRotateCall)
			}
			if tt.wantRotateCall == 0 {
				return
			}
			if s.refreshTokenRepo.rotateCalls[0].oldHash != "hashed:plain" {
				t.Errorf("Refresh() called Rotate with oldHash = %v, want %v", s.refreshTokenRepo.rotateCalls[0].oldHash, "hashed:plain")
			}
			if s.refreshTokenRepo.rotateCalls[0].newHash != s.tokens.genHash {
				t.Errorf("Refresh() called Rotate with newHash = %v, want %v", s.refreshTokenRepo.rotateCalls[0].newHash, s.tokens.genHash)
			}
			if s.refreshTokenRepo.rotateCalls[0].ttl != s.refreshTokenTTL {
				t.Errorf("Refresh() called Rotate with ttl = %v, want %v", s.refreshTokenRepo.rotateCalls[0].ttl, s.refreshTokenTTL)
			}
			if len(s.refreshTokenRepo.findByHashArgs) != 0 {
				t.Errorf("Refresh() called FindByHash when it should not have")
			}
			if len(s.refreshTokenRepo.revokeAllArgs) != 0 {
				t.Errorf("Refresh() called RevokeAllForUser when it should not have")
			}

			if len(s.userRepo.findByIDArgs) != 1 {
				t.Fatalf("Refresh() did not call FindByID when expected")
			}
			if s.userRepo.findByIDArgs[0] != testUserID {
				t.Errorf("Refresh() called FindByID with userID = %v, want %v", s.userRepo.findByIDArgs[0], testUserID)
			}
			if len(s.refreshTokenRepo.revokeByHashArgs) != tt.wantRevokeByHashCall {
				t.Fatalf("Refresh() called RevokeByHash with %d calls, want %d", len(s.refreshTokenRepo.revokeByHashArgs), tt.wantRevokeByHashCall)
			}
			if tt.wantRevokeByHashCall > 0 && s.refreshTokenRepo.revokeByHashArgs[0] != s.tokens.genHash {
				t.Errorf("Refresh() called RevokeByHash with hash = %v, want %v", s.refreshTokenRepo.revokeByHashArgs[0], s.tokens.genHash)
			}
			if len(s.tokens.accessCalls) != tt.wantAccessCall {
				t.Fatalf("Refresh() called IssueAccessToken with %d args, want %d", len(s.tokens.accessCalls), tt.wantAccessCall)
			}
			if tt.wantAccessCall == 0 {
				return
			}
			if s.tokens.accessCalls[0].userID != testUserID.String() {
				t.Errorf("Refresh() called IssueAccessToken with userID = %v, want %v", s.tokens.accessCalls[0].userID, testUserID.String())
			}
			if s.tokens.accessCalls[0].role != "user" {
				t.Errorf("Refresh() called IssueAccessToken with role = %v, want %v", s.tokens.accessCalls[0].role, "user")
			}
			if tkn.AccessToken != s.tokens.accessToken {
				t.Errorf("Refresh() returned incorrect AccessToken")
			}
			if tkn.RefreshToken != s.tokens.genPlain {
				t.Errorf("Refresh() returned incorrect RefreshToken")
			}
		})
	}
}

func TestUserService_Refresh_InvalidToken(t *testing.T) {
	tests := []struct {
		name string

		mutateToken            func(t *domain.RefreshToken)
		findByHashErr          error
		wantRevokeAllCalls     int
		wantRevokedByHashCalls int
	}{
		{
			name: "token expired and rotated",
			mutateToken: func(t *domain.RefreshToken) {
				t.ReplacedByTokenID = &replacedByID
				t.ExpiresAt = timeDB.Add(-1 * time.Hour)
				t.RevokedAt = &timeDB
			},
			findByHashErr:          nil,
			wantRevokeAllCalls:     0,
			wantRevokedByHashCalls: 0,
		},
		{
			name: "token rotated and revoked in grace window",
			mutateToken: func(t *domain.RefreshToken) {
				t.ReplacedByTokenID = &replacedByID
				t.ExpiresAt = timeDB.Add(1 * time.Hour)
				revokedAt := timeDB.Add(-2 * time.Second)
				t.RevokedAt = &revokedAt
			},
			findByHashErr:          nil,
			wantRevokeAllCalls:     0,
			wantRevokedByHashCalls: 0,
		},
		{
			name: "token rotated and not revoked in grace window",
			mutateToken: func(t *domain.RefreshToken) {
				t.ReplacedByTokenID = &replacedByID
				t.ExpiresAt = timeDB.Add(1 * time.Hour)
				revokedAt := timeDB.Add(-60 * time.Second)
				t.RevokedAt = &revokedAt
			},
			findByHashErr:          nil,
			wantRevokeAllCalls:     1,
			wantRevokedByHashCalls: 0,
		},
		{
			name: "logout with revoked token",
			mutateToken: func(t *domain.RefreshToken) {
				t.ReplacedByTokenID = nil
				t.ExpiresAt = timeDB.Add(1 * time.Hour)
				revokedAt := timeDB.Add(-1 * time.Second)
				t.RevokedAt = &revokedAt
			},
			findByHashErr:          nil,
			wantRevokeAllCalls:     0,
			wantRevokedByHashCalls: 0,
		},
		{
			name: "token expired",
			mutateToken: func(t *domain.RefreshToken) {
				t.ReplacedByTokenID = nil
				t.ExpiresAt = timeDB.Add(-1 * time.Second)
				t.RevokedAt = nil
			},
			findByHashErr:          nil,
			wantRevokeAllCalls:     0,
			wantRevokedByHashCalls: 0,
		},
		{
			name:                   "token not found",
			mutateToken:            func(t *domain.RefreshToken) {},
			findByHashErr:          domain.ErrNotFound,
			wantRevokeAllCalls:     0,
			wantRevokedByHashCalls: 0,
		},
		{
			name:                   "alive token that failed capture",
			mutateToken:            func(t *domain.RefreshToken) {},
			findByHashErr:          nil,
			wantRevokeAllCalls:     0,
			wantRevokedByHashCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStand(t, 24*time.Hour, 15*time.Second)
			refreshToken := newValidRefreshToken()
			tt.mutateToken(refreshToken)
			s.refreshTokenRepo.findByHashToken = refreshToken
			s.refreshTokenRepo.findByHashDBNow = timeDB
			s.refreshTokenRepo.findByHashErr = tt.findByHashErr
			s.refreshTokenRepo.rotateErr = domain.ErrTokenNotUsable

			tkn, err := s.svc.Refresh(context.Background(), "plain")

			if tkn != nil {
				t.Errorf("Refresh() returned token when it should not have")
			}
			if !errors.Is(err, domain.ErrInvalidRefreshToken) {
				t.Fatalf("Refresh() error = %v, wantErr %v", err, domain.ErrInvalidRefreshToken)
			}
			if len(s.refreshTokenRepo.findByHashArgs) != 1 {
				t.Fatalf("Refresh() did not call FindByHash when expected")
			}
			if s.refreshTokenRepo.findByHashArgs[0] != "hashed:plain" {
				t.Errorf("Refresh() called FindByHash with tokenHash = %v, want %v", s.refreshTokenRepo.findByHashArgs[0], "hashed:plain")
			}
			if len(s.refreshTokenRepo.revokeAllArgs) != tt.wantRevokeAllCalls {
				t.Fatalf("Refresh() called RevokeAllForUser with %d calls, want %d", len(s.refreshTokenRepo.revokeAllArgs), tt.wantRevokeAllCalls)
			}
			if tt.wantRevokeAllCalls > 0 && s.refreshTokenRepo.revokeAllArgs[0] != testUserID {
				t.Errorf("Refresh() called RevokeAllForUser with userID = %v, want %v", s.refreshTokenRepo.revokeAllArgs[0], testUserID)
			}
			if len(s.refreshTokenRepo.revokeByHashArgs) != tt.wantRevokedByHashCalls {
				t.Fatalf("Refresh() called RevokeByHash with %d calls, want %d", len(s.refreshTokenRepo.revokeByHashArgs), tt.wantRevokedByHashCalls)
			}

		})
	}
}

func generatePasswordHash(password string) (string, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	return string(passwordHash), err
}

const wantEmail = "abc@mail.ru"
const wantPassword = "correctpassword"

func validUserParams() *domain.UserParams {
	return &domain.UserParams{
		Email:    wantEmail,
		Password: wantPassword,
	}
}

func newTestUser() *domain.User {
	passwordHash, err := generatePasswordHash(wantPassword)
	if err != nil {
		panic(err)
	}
	return &domain.User{
		Id:           testUserID,
		Email:        wantEmail,
		PasswordHash: passwordHash,
		Role:         "user",
		IsActive:     true,
	}
}

func TestUserService_Login(t *testing.T) {
	tests := []struct {
		name             string
		mutateUserParams func(u *domain.UserParams)
		mutateStand      func(s *stand)
		passwordDB       string
		wantEmail        string
		wantErr          error
		wantToken        bool
		wantFindByCall   int
		wantCreateCall   int
	}{
		{
			name: "valid user",
			mutateUserParams: func(u *domain.UserParams) {
				u.Email = "  AbC@maIl.RU  "
			},
			wantEmail:      wantEmail,
			passwordDB:     wantPassword,
			wantErr:        nil,
			wantToken:      true,
			wantFindByCall: 1,
			wantCreateCall: 1,
		},
		{
			name: "too short password",
			mutateUserParams: func(u *domain.UserParams) {
				u.Password = "123"
			},
			wantEmail:      wantEmail,
			passwordDB:     "123",
			wantErr:        domain.ErrValidate,
			wantToken:      false,
			wantFindByCall: 0,
			wantCreateCall: 0,
		},
		{
			name: "invalid password",
			mutateUserParams: func(u *domain.UserParams) {
				u.Password = "wrongpassword"
			},
			wantEmail:      wantEmail,
			passwordDB:     wantPassword,
			wantErr:        domain.ErrInvalidCredentials,
			wantToken:      false,
			wantFindByCall: 1,
			wantCreateCall: 0,
		},
		{
			name: "email not found",
			mutateUserParams: func(u *domain.UserParams) {
				u.Email = "notfound@mail.ru"
			},
			mutateStand: func(s *stand) {
				s.userRepo.findByEmailErr = domain.ErrNotFound
			},
			wantEmail:      "notfound@mail.ru",
			passwordDB:     wantPassword,
			wantErr:        domain.ErrInvalidCredentials,
			wantToken:      false,
			wantFindByCall: 1,
			wantCreateCall: 0,
		},
		{
			name: "invalid email",
			mutateUserParams: func(u *domain.UserParams) {
				u.Email = "invalidemail"
			},
			wantEmail:      wantEmail,
			passwordDB:     wantPassword,
			wantErr:        domain.ErrValidate,
			wantToken:      false,
			wantFindByCall: 0,
			wantCreateCall: 0,
		},
		{
			name: "inactive user",
			mutateStand: func(s *stand) {
				s.userRepo.findByEmailUser.IsActive = false
			},
			wantEmail:      wantEmail,
			passwordDB:     wantPassword,
			wantErr:        domain.ErrAccountDisabled,
			wantToken:      false,
			wantFindByCall: 1,
			wantCreateCall: 0,
		},
		{
			name: "error from user repo",
			mutateStand: func(s *stand) {
				s.userRepo.findByEmailErr = repoErr
			},
			wantEmail:      wantEmail,
			passwordDB:     wantPassword,
			wantErr:        domain.ErrRepositoryUnavailable,
			wantToken:      false,
			wantFindByCall: 1,
			wantCreateCall: 0,
		},
		{
			name: "error generating refresh token",
			mutateStand: func(s *stand) {
				s.tokens.genErr = genErr
			},
			wantEmail:      wantEmail,
			passwordDB:     wantPassword,
			wantErr:        genErr,
			wantToken:      false,
			wantFindByCall: 1,
			wantCreateCall: 0,
		},
		{
			name: "error creating refresh token",
			mutateStand: func(s *stand) {
				s.refreshTokenRepo.createErr = createErr
			},
			wantEmail:      wantEmail,
			passwordDB:     wantPassword,
			wantErr:        createErr,
			wantToken:      false,
			wantFindByCall: 1,
			wantCreateCall: 1,
		},
		{
			name: "error issuing access token",
			mutateStand: func(s *stand) {
				s.tokens.accessErr = accessErr
			},
			wantEmail:      wantEmail,
			passwordDB:     wantPassword,
			wantErr:        accessErr,
			wantToken:      false,
			wantFindByCall: 1,
			wantCreateCall: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStand(t, 24*time.Hour, 15*time.Second)
			s.tokens.genPlain = "new-plain"
			s.tokens.genHash = "hashed:new-plain"
			s.tokens.accessToken = "access-token"
			user := validUserParams()
			s.userRepo.findByEmailUser = newTestUser()
			if tt.mutateUserParams != nil {
				tt.mutateUserParams(user)
			}
			if tt.mutateStand != nil {
				tt.mutateStand(s)
			}

			tkn, err := s.svc.Login(context.Background(), user)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Login() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(s.userRepo.findByEmailArgs) != tt.wantFindByCall {
				t.Fatalf("Login() called FindByEmail with %d calls, want %d", len(s.userRepo.findByEmailArgs), tt.wantFindByCall)
			}
			if tt.wantFindByCall > 0 && s.userRepo.findByEmailArgs[0] != tt.wantEmail {
				t.Errorf("Login() called FindByEmail with email = %v, want %v", s.userRepo.findByEmailArgs[0], tt.wantEmail)
			}
			if len(s.refreshTokenRepo.createCalls) != tt.wantCreateCall {
				t.Fatalf("Login() did not call Create refresh token when expected")
			}
			if tt.wantCreateCall > 0 {
				if s.refreshTokenRepo.createCalls[0].userID != testUserID {
					t.Errorf("Login() called Create refresh token with userID = %v, want %v", s.refreshTokenRepo.createCalls[0].userID, testUserID)
				}
				if s.refreshTokenRepo.createCalls[0].tokenHash != s.tokens.genHash {
					t.Errorf("Login() called Create refresh token with tokenHash = %v, want %v", s.refreshTokenRepo.createCalls[0].tokenHash, s.tokens.genHash)
				}
				if s.refreshTokenRepo.createCalls[0].ttl != s.refreshTokenTTL {
					t.Errorf("Login() called Create refresh token with ttl = %v, want %v", s.refreshTokenRepo.createCalls[0].ttl, s.refreshTokenTTL)
				}
			}
			if !tt.wantToken {
				return
			}
			if len(s.tokens.accessCalls) != 1 {
				t.Fatalf("Login() did not call IssueAccessToken when expected")
			}
			if s.tokens.accessCalls[0].userID != testUserID.String() {
				t.Errorf("Login() called IssueAccessToken with userID = %v, want %v", s.tokens.accessCalls[0].userID, testUserID.String())
			}
			if s.tokens.accessCalls[0].role != "user" {
				t.Errorf("Login() called IssueAccessToken with role = %v, want %v", s.tokens.accessCalls[0].role, "user")
			}

			if tkn == nil {
				t.Fatalf("Login() returned nil token")
			}
			if tkn.AccessToken != s.tokens.accessToken {
				t.Errorf("Login() returned AccessToken = %v, want %v", tkn.AccessToken, s.tokens.accessToken)
			}
			if tkn.RefreshToken != s.tokens.genPlain {
				t.Errorf("Login() returned RefreshToken = %v, want %v", tkn.RefreshToken, s.tokens.genPlain)
			}
		})
	}
}

func TestUserService_Create(t *testing.T) {
	tests := []struct {
		name             string
		mutateUserParams func(u *domain.UserParams)
		mutateStand      func(s *stand)
		wantEmail        string
		wantPassword     string
		wantErr          error
		wantCreateCall   int
		wantUser         bool
	}{
		{
			name: "success",
			mutateUserParams: func(u *domain.UserParams) {
				u.Email = "  AbC@mail.Ru "
			},
			wantEmail:      "abc@mail.ru",
			wantPassword:   wantPassword,
			wantErr:        nil,
			wantCreateCall: 1,
			wantUser:       true,
		},
		{
			name: "invalid email",
			mutateUserParams: func(u *domain.UserParams) {
				u.Email = "invalidemail"
			},
			wantEmail:      "abc@mail.ru",
			wantPassword:   wantPassword,
			wantErr:        domain.ErrValidate,
			wantCreateCall: 0,
		},
		{
			name: "too short password",
			mutateUserParams: func(u *domain.UserParams) {
				u.Password = "123"
			},
			wantEmail:      "abc@mail.ru",
			wantPassword:   "123",
			wantErr:        domain.ErrValidate,
			wantCreateCall: 0,
		},
		{
			name: "duplicate email",
			mutateStand: func(s *stand) {
				s.userRepo.createErr = domain.ErrConflict
			},
			wantEmail:      "abc@mail.ru",
			wantPassword:   wantPassword,
			wantErr:        domain.ErrConflict,
			wantCreateCall: 1,
		},
		{
			name: "error from user repo",
			mutateStand: func(s *stand) {
				s.userRepo.createErr = repoErr
			},
			wantEmail:      "abc@mail.ru",
			wantPassword:   wantPassword,
			wantErr:        repoErr,
			wantCreateCall: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStand(t, 24*time.Hour, 15*time.Second)

			user := validUserParams()
			s.userRepo.createUser = newTestUser()
			if tt.mutateUserParams != nil {
				tt.mutateUserParams(user)
			}
			if tt.mutateStand != nil {
				tt.mutateStand(s)
			}

			u, err := s.svc.Create(context.Background(), user)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(s.userRepo.createUserCalls) != tt.wantCreateCall {
				t.Fatalf("Create() called Create with %d calls, want %d", len(s.userRepo.createUserCalls), tt.wantCreateCall)
			}

			if tt.wantCreateCall > 0 {
				if s.userRepo.createUserCalls[0].email != tt.wantEmail {
					t.Errorf("Create() called Create with email = %v, want %v", s.userRepo.createUserCalls[0].email, tt.wantEmail)
				}
				if err := bcrypt.CompareHashAndPassword([]byte(s.userRepo.createUserCalls[0].password), []byte(user.Password)); err != nil {
					t.Errorf("password sent to repository is not a valid bcrypt hash of the input")
				}
			}
			if tt.wantPassword != user.Password {
				t.Errorf("Create() user have password = %v, want %v", user.Password, tt.wantPassword)
			}
			if tt.wantUser {
				if u == nil {
					t.Fatalf("Create() returned nil user")
				}
				if u.Id != testUserID {
					t.Errorf("Create() return user with id = %v, want %v", u.Id, testUserID)
				}
				return
			}
			if u != nil {
				t.Fatalf("Create() returned user")
			}

		})

	}
}
