package settings_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/domain/settings"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func build(t *testing.T, f settings.SettingsFactory) settings.Settings {
	t.Helper()
	s, err := f.Build()
	require.NoError(t, err)
	return s
}

func valid(t *testing.T) settings.SettingsFactory {
	t.Helper()
	f := settings.NewSettingsFactory()
	require.NoError(t, f.Name("Trend Kids JP"))
	require.NoError(t, f.City("João Pessoa · PB"))
	require.NoError(t, f.Description("Crocs infantis"))
	require.NoError(t, f.WhatsApp("(83) 99999-1234"))
	require.NoError(t, f.Phone("(83) 3222-1234"))
	require.NoError(t, f.Instagram("@trendkidsjp"))
	return f
}

func TestSettings_BuildsWithNormalizedContacts(t *testing.T) {
	s := build(t, valid(t))

	assert.Equal(t, "Trend Kids JP", s.Name())
	assert.Equal(t, "João Pessoa · PB", s.City())
	assert.Equal(t, "83999991234", s.WhatsApp(), "guarda só os dígitos nacionais")
	assert.Equal(t, "8332221234", s.Phone())
	assert.Equal(t, "trendkidsjp", s.Instagram(), "guarda só o usuário, sem @")
}

func TestSettings_ContactsAreOptional(t *testing.T) {
	f := settings.NewSettingsFactory()
	require.NoError(t, f.Name("Trend Kids JP"))
	require.NoError(t, f.City("João Pessoa"))
	for _, err := range []error{f.WhatsApp(""), f.WhatsApp("   "), f.Phone(""), f.Instagram(""), f.Description("")} {
		require.NoError(t, err, "vazio = ainda não configurado")
	}
	s := build(t, f)
	assert.Empty(t, s.WhatsApp())
	assert.Empty(t, s.Phone())
	assert.Empty(t, s.Instagram())
}

func TestInstagram_AcceptsHandleOrProfileUrl(t *testing.T) {
	for in, want := range map[string]string{
		"@Trend.Kids_JP":                         "trend.kids_jp",
		"trendkidsjp":                            "trendkidsjp",
		"https://www.instagram.com/trendkidsjp/": "trendkidsjp",
		"instagram.com/trendkidsjp?igsh=abc123":  "trendkidsjp",
		"http://instagram.com/trendkidsjp":       "trendkidsjp",
		"  @trendkidsjp  ":                       "trendkidsjp",
	} {
		f := settings.NewSettingsFactory()
		require.NoError(t, f.Instagram(in), in)
		require.NoError(t, f.Name("Loja"))
		require.NoError(t, f.City("JP"))
		assert.Equal(t, want, build(t, f).Instagram(), in)
	}
}

func TestInstagram_RejectsInvalid(t *testing.T) {
	for _, in := range []string{"@", "ab cd", "@@x", "a/b", "https://outrosite.com/loja", "javascript:alert(1)", strings.Repeat("a", 31), "joão"} {
		err := settings.NewSettingsFactory().Instagram(in)
		var req httperr.RequiredError
		assert.True(t, errors.As(err, &req), in)
	}
}

func TestSettings_RejectsInvalidFields(t *testing.T) {
	tests := map[string]func(f settings.SettingsFactory) error{
		"nome vazio":          func(f settings.SettingsFactory) error { return f.Name("  ") },
		"nome de 1 letra":     func(f settings.SettingsFactory) error { return f.Name("T") },
		"nome gigante":        func(f settings.SettingsFactory) error { return f.Name(strings.Repeat("a", 81)) },
		"cidade vazia":        func(f settings.SettingsFactory) error { return f.City("") },
		"cidade gigante":      func(f settings.SettingsFactory) error { return f.City(strings.Repeat("a", 81)) },
		"descrição gigante":   func(f settings.SettingsFactory) error { return f.Description(strings.Repeat("a", 301)) },
		"whatsapp inválido":   func(f settings.SettingsFactory) error { return f.WhatsApp("123") },
		"whatsapp com letras": func(f settings.SettingsFactory) error { return f.WhatsApp("83 9999x-1234") },
		"telefone inválido":   func(f settings.SettingsFactory) error { return f.Phone("00999991234") },
	}
	for name, call := range tests {
		err := call(settings.NewSettingsFactory())
		var req httperr.RequiredError
		assert.True(t, errors.As(err, &req), name)
	}
}

func TestBuild_RequiresNameAndCity(t *testing.T) {
	_, err := settings.NewSettingsFactory().Build()
	assert.Error(t, err)
}

func TestInstagramURL(t *testing.T) {
	assert.Equal(t, "https://www.instagram.com/trendkidsjp", settings.InstagramURL("trendkidsjp"))
	assert.Equal(t, "", settings.InstagramURL(""))
}
