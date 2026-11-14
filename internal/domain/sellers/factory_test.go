package sellers_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Logstay/trend-kids-api/internal/domain/sellers"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func valid(t *testing.T) sellers.Seller {
	t.Helper()
	f := sellers.NewSellerFactory()
	require.NoError(t, f.Name("  Ana Paula "))
	require.NoError(t, f.WhatsApp("+55 (83) 99999-1234"))
	s, err := f.Build()
	require.NoError(t, err)
	return s
}

func TestSeller_BuildsActiveWithNormalizedWhatsApp(t *testing.T) {
	s := valid(t)

	assert.Equal(t, "Ana Paula", s.Name())
	assert.Equal(t, "83999991234", s.WhatsApp(), "guarda só os dígitos nacionais")
	assert.True(t, s.Active(), "nasce ativa")
	assert.NotEmpty(t, s.UID())
}

func TestSeller_RejectsInvalidFields(t *testing.T) {
	tests := map[string]func(f sellers.SellerFactory) error{
		"nome vazio":         func(f sellers.SellerFactory) error { return f.Name("  ") },
		"nome de 1 letra":    func(f sellers.SellerFactory) error { return f.Name("A") },
		"nome gigante":       func(f sellers.SellerFactory) error { return f.Name(strings.Repeat("a", 61)) },
		"whatsapp vazio":     func(f sellers.SellerFactory) error { return f.WhatsApp("") },
		"whatsapp inválido":  func(f sellers.SellerFactory) error { return f.WhatsApp("123") },
		"whatsapp com letra": func(f sellers.SellerFactory) error { return f.WhatsApp("83 9999x-1234") },
	}
	for name, call := range tests {
		err := call(sellers.NewSellerFactory())
		var req httperr.RequiredError
		assert.True(t, errors.As(err, &req), name)
	}
}

func TestBuild_RequiresNameAndWhatsApp(t *testing.T) {
	_, err := sellers.NewSellerFactory().Build()
	assert.Error(t, err)

	f := sellers.NewSellerFactory()
	require.NoError(t, f.Name("Ana"))
	_, err = f.Build()
	assert.Error(t, err, "sem whatsapp")
}

func TestFactoryFrom_KeepsIdentityAndCanDeactivate(t *testing.T) {
	old := sellers.Restore(sellers.Snapshot{UID: "u-1", Name: "Ana", WhatsApp: "83999991234", Active: true, CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)})

	f := sellers.NewSellerFactoryFrom(old)
	require.NoError(t, f.Name("Ana Paula"))
	f.Active(false)
	updated, err := f.Build()

	require.NoError(t, err)
	assert.Equal(t, "u-1", updated.UID())
	assert.Equal(t, old.CreatedAt(), updated.CreatedAt())
	assert.Equal(t, "Ana Paula", updated.Name())
	assert.False(t, updated.Active())
	assert.Equal(t, "Ana", old.Name(), "o original não é alterado")
	assert.True(t, old.Active())
}
