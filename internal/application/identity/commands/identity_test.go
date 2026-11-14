package commands_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Logstay/trend-kids-api/internal/adapters/security"
	"github.com/Logstay/trend-kids-api/internal/application/identity/commands"
	ports "github.com/Logstay/trend-kids-api/internal/ports/identity"
	"github.com/Logstay/trend-kids-api/internal/ports/identity/identitytest"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const password = "uma-senha-bem-longa"

type env struct {
	repo   *identitytest.Repo
	create commands.CreateAdminHandler
	login  commands.LoginHandler
	tokens *security.TokenService
}

func newEnv(t *testing.T) env {
	t.Helper()
	repo := &identitytest.Repo{}
	hasher := security.NewArgon2Hasher()
	tokens, err := security.NewTokenService("0123456789abcdef0123456789abcdef", time.Hour, time.Now)
	require.NoError(t, err)
	return env{
		repo:   repo,
		tokens: tokens,
		create: commands.CreateAdminHandler{Repository: repo, Hasher: hasher},
		login:  commands.LoginHandler{Repository: repo, Hasher: hasher, Tokens: tokens},
	}
}

func (e env) seedAdmin(t *testing.T) {
	t.Helper()
	require.NoError(t, e.create.Handle(context.Background(), ports.CreateAdminCommand{Email: "dono@trendkids.com.br", Password: password}))
}

func TestCreateAdmin_StoresHashNeverThePassword(t *testing.T) {
	e := newEnv(t)
	e.seedAdmin(t)

	require.Len(t, e.repo.Admins, 1)
	assert.NotContains(t, e.repo.Admins[0].PasswordHash(), password)
	assert.Contains(t, e.repo.Admins[0].PasswordHash(), "$argon2id$")
}

func TestCreateAdmin_RejectsShortPasswordAndSavesNothing(t *testing.T) {
	e := newEnv(t)

	err := e.create.Handle(context.Background(), ports.CreateAdminCommand{Email: "dono@x.com", Password: "curta"})

	var req httperr.RequiredError
	assert.True(t, errors.As(err, &req))
	assert.Empty(t, e.repo.Admins)
}

func TestCreateAdmin_RejectsDuplicateEmail(t *testing.T) {
	e := newEnv(t)
	e.seedAdmin(t)

	err := e.create.Handle(context.Background(), ports.CreateAdminCommand{Email: "DONO@trendkids.com.br", Password: password})

	var conflict httperr.ConflictError
	assert.True(t, errors.As(err, &conflict))
	assert.Len(t, e.repo.Admins, 1)
}

func TestLogin_ValidCredentialsIssueTokenForThatAdmin(t *testing.T) {
	e := newEnv(t)
	e.seedAdmin(t)

	s, err := e.login.Handle(context.Background(), ports.LoginCommand{Email: "  Dono@TrendKids.com.br ", Password: password})

	require.NoError(t, err)
	uid, err := e.tokens.Parse(s.Token)
	require.NoError(t, err)
	assert.Equal(t, e.repo.Admins[0].UID(), uid)
	assert.True(t, s.ExpiresAt.After(time.Now()))
}

func TestLogin_WrongPasswordAndUnknownEmailLookTheSame(t *testing.T) {
	e := newEnv(t)
	e.seedAdmin(t)

	_, wrongPass := e.login.Handle(context.Background(), ports.LoginCommand{Email: "dono@trendkids.com.br", Password: "senha-errada-123"})
	_, unknown := e.login.Handle(context.Background(), ports.LoginCommand{Email: "ninguem@x.com", Password: password})

	var u1, u2 httperr.UnauthorizedError
	require.True(t, errors.As(wrongPass, &u1))
	require.True(t, errors.As(unknown, &u2))
	assert.Equal(t, wrongPass.Error(), unknown.Error(), "não pode revelar se o e-mail existe")
}
