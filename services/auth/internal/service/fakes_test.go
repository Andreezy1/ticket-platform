package service_test

import (
	"context"
	"errors"
	"time"

	"github.com/Andreezy1/ticket-platform/services/auth/internal/domain"
	"github.com/Andreezy1/ticket-platform/services/auth/internal/service"
	"github.com/google/uuid"
)

var repoErr = errors.New("repo error")
var genErr = errors.New("gen error")
var createErr = errors.New("create error")
var accessErr = errors.New("access error")

type createUserCall struct {
	email    string
	password string
}

type fakeUserRepo struct {
	findByIDUser *domain.User
	findByIDErr  error
	findByIDArgs []uuid.UUID

	findByEmailUser *domain.User
	findByEmailErr  error
	findByEmailArgs []string

	createUser      *domain.User
	createErr       error
	createUserCalls []createUserCall
}

func (u *fakeUserRepo) Create(ctx context.Context, user *domain.UserParams) (*domain.User, error) {
	u.createUserCalls = append(u.createUserCalls, createUserCall{email: user.Email, password: user.Password})
	return u.createUser, u.createErr
}
func (u *fakeUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	u.findByEmailArgs = append(u.findByEmailArgs, email)
	return u.findByEmailUser, u.findByEmailErr
}
func (u *fakeUserRepo) FindByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	u.findByIDArgs = append(u.findByIDArgs, userID)
	return u.findByIDUser, u.findByIDErr
}

var _ service.UserRepository = (*fakeUserRepo)(nil)

type rotateCall struct {
	oldHash string
	newHash string
	ttl     time.Duration
}

type createCall struct {
	userID    uuid.UUID
	tokenHash string
	ttl       time.Duration
}

type fakeTokenRepo struct {
	rotateUserID uuid.UUID
	rotateErr    error
	rotateCalls  []rotateCall

	findByHashToken *domain.RefreshToken
	findByHashErr   error
	findByHashArgs  []string
	findByHashDBNow time.Time

	revokeAllArgs []uuid.UUID
	revokeAllErr  error

	revokeByHashArgs []string
	revokeByHashErr  error

	createCalls []createCall
	createErr   error
}

func (t *fakeTokenRepo) Create(ctx context.Context, userID uuid.UUID, tokenHash string, ttl time.Duration) error {
	t.createCalls = append(t.createCalls, createCall{userID: userID, tokenHash: tokenHash, ttl: ttl})
	return t.createErr
}
func (t *fakeTokenRepo) FindByHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, time.Time, error) {
	t.findByHashArgs = append(t.findByHashArgs, tokenHash)
	return t.findByHashToken, t.findByHashDBNow, t.findByHashErr
}
func (t *fakeTokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	t.revokeAllArgs = append(t.revokeAllArgs, userID)
	return t.revokeAllErr
}
func (t *fakeTokenRepo) RevokeByHash(ctx context.Context, tokenHash string) error {
	t.revokeByHashArgs = append(t.revokeByHashArgs, tokenHash)
	return t.revokeByHashErr
}
func (t *fakeTokenRepo) Rotate(ctx context.Context, oldHash string, newHash string, ttl time.Duration) (userID uuid.UUID, err error) {
	t.rotateCalls = append(t.rotateCalls, rotateCall{oldHash: oldHash, newHash: newHash, ttl: ttl})
	return t.rotateUserID, t.rotateErr
}

var _ service.RefreshTokenRepository = (*fakeTokenRepo)(nil)

type accessCall struct {
	userID string
	role   string
}

type fakeManager struct {
	genPlain string
	genHash  string
	genErr   error

	accessToken string
	accessErr   error
	accessCalls []accessCall
}

var _ service.ManagerToken = (*fakeManager)(nil)

func (m *fakeManager) IssueAccessToken(userID, role string) (string, error) {
	m.accessCalls = append(m.accessCalls, accessCall{userID: userID, role: role})
	return m.accessToken, m.accessErr
}
func (m *fakeManager) GenerateRefreshToken() (plain string, hash string, err error) {
	return m.genPlain, m.genHash, m.genErr
}
func (m *fakeManager) HashRefreshToken(plain string) string {
	return "hashed:" + plain
}
