package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/application/settings/commands"
	ports "github.com/Logstay/trend-kids-api/internal/ports/settings"
	"github.com/Logstay/trend-kids-api/internal/ports/settings/settingstest"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cmd() ports.UpdateSettings {
	return ports.UpdateSettings{
		Name: "Trend Kids JP", City: "João Pessoa · PB", Description: "Crocs infantis",
		WhatsApp: "(83) 99999-1234", Phone: "(83) 3222-1234", Instagram: "@trendkidsjp", AdminUID: "admin-1",
	}
}

func TestUpdate_StoresNormalizedValuesAndWhoChangedThem(t *testing.T) {
	repo := settingstest.New()

	got, err := commands.UpdateHandler{Repository: repo}.Handle(context.Background(), cmd())

	require.NoError(t, err)
	assert.Equal(t, "83999991234", got.WhatsApp())
	assert.Equal(t, "trendkidsjp", got.Instagram())
	assert.Equal(t, []string{"admin-1"}, repo.UpdatedBy)
	assert.Equal(t, "83999991234", repo.Current.WhatsApp())
}

func TestUpdate_InvalidFieldChangesNothing(t *testing.T) {
	repo := settingstest.New()
	for name, mutate := range map[string]func(c *ports.UpdateSettings){
		"whatsapp inválido":  func(c *ports.UpdateSettings) { c.WhatsApp = "123" },
		"instagram inválido": func(c *ports.UpdateSettings) { c.Instagram = "https://outro.com/x" },
		"nome vazio":         func(c *ports.UpdateSettings) { c.Name = " " },
		"cidade vazia":       func(c *ports.UpdateSettings) { c.City = "" },
	} {
		c := cmd()
		mutate(&c)
		_, err := commands.UpdateHandler{Repository: repo}.Handle(context.Background(), c)
		var req httperr.RequiredError
		assert.True(t, errors.As(err, &req), name)
	}
	assert.Equal(t, 0, repo.Updates, "nenhuma atualização chegou ao repositório")
	assert.Empty(t, repo.Current.WhatsApp())
}

func TestUpdate_EmptyContactsRemoveThem(t *testing.T) {
	repo := settingstest.New()
	h := commands.UpdateHandler{Repository: repo}
	_, err := h.Handle(context.Background(), cmd())
	require.NoError(t, err)

	c := cmd()
	c.WhatsApp, c.Phone, c.Instagram = "", "", ""
	got, err := h.Handle(context.Background(), c)

	require.NoError(t, err)
	assert.Empty(t, got.WhatsApp())
	assert.Empty(t, got.Phone())
	assert.Empty(t, got.Instagram())
}
