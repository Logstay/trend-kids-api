package identity_test

import (
	"errors"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/domain/identity"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminFactory_BuildsAdminWithNormalizedEmail(t *testing.T) {
	f := identity.NewAdminFactory()
	require.NoError(t, f.Email("  Dono@TrendKids.com.br "))
	require.NoError(t, f.PasswordHash("$argon2id$hash"))

	a := f.Build()

	assert.Equal(t, "dono@trendkids.com.br", a.Email())
	assert.Equal(t, "$argon2id$hash", a.PasswordHash())
	assert.NotEmpty(t, a.UID())
}

func TestAdminFactory_RejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name string
		call func(f identity.AdminFactory) error
	}{
		{"e-mail vazio", func(f identity.AdminFactory) error { return f.Email("") }},
		{"e-mail sem arroba", func(f identity.AdminFactory) error { return f.Email("dono.trendkids.com") }},
		{"e-mail com nome", func(f identity.AdminFactory) error { return f.Email("Dono <dono@x.com>") }},
		{"hash vazio", func(f identity.AdminFactory) error { return f.PasswordHash("") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call(identity.NewAdminFactory())
			var req httperr.RequiredError
			assert.True(t, errors.As(err, &req), "veio %v", err)
		})
	}
}
