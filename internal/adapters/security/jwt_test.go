package security_test

import (
	"testing"
	"time"

	"github.com/Logstay/trend-kids-api/internal/adapters/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secret = "0123456789abcdef0123456789abcdef"

func TestJWT_IssueAndParseRoundTrip(t *testing.T) {
	svc, err := security.NewTokenService(secret, time.Hour, time.Now)
	require.NoError(t, err)

	token, expires, err := svc.Issue("admin-1")
	require.NoError(t, err)
	uid, err := svc.Parse(token)

	require.NoError(t, err)
	assert.Equal(t, "admin-1", uid)
	assert.WithinDuration(t, time.Now().Add(time.Hour), expires, 5*time.Second)
}

func TestJWT_RejectsExpiredToken(t *testing.T) {
	now := time.Now()
	issuer, _ := security.NewTokenService(secret, time.Hour, func() time.Time { return now })
	token, _, _ := issuer.Issue("admin-1")

	later, _ := security.NewTokenService(secret, time.Hour, func() time.Time { return now.Add(2 * time.Hour) })
	_, err := later.Parse(token)

	assert.Error(t, err)
}

func TestJWT_RejectsTamperedOrForeignToken(t *testing.T) {
	svc, _ := security.NewTokenService(secret, time.Hour, time.Now)
	other, _ := security.NewTokenService("ffffffffffffffffffffffffffffffff", time.Hour, time.Now)
	foreign, _, _ := other.Issue("admin-1")
	good, _, _ := svc.Issue("admin-1")

	for name, tok := range map[string]string{
		"assinado com outro segredo": foreign,
		"adulterado":                 good[:len(good)-2] + "xx",
		"lixo":                       "nao.e.jwt",
		"vazio":                      "",
	} {
		_, err := svc.Parse(tok)
		assert.Error(t, err, name)
	}
}

func TestJWT_RejectsShortSecret(t *testing.T) {
	_, err := security.NewTokenService("curto", time.Hour, time.Now)
	assert.Error(t, err)
}
