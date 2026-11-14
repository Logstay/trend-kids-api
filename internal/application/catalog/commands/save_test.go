package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/application/catalog/commands"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	"github.com/Logstay/trend-kids-api/internal/ports/catalog/catalogtest"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validCmd() ports.SaveProduct {
	return ports.SaveProduct{Name: "Classic Clog Rosa", Category: "Classic", PriceCents: 18990, Stock: 3}
}

func TestSave_PersistsValidProduct(t *testing.T) {
	repo := &catalogtest.Repo{}
	created, err := commands.SaveHandler{Repository: repo}.Handle(context.Background(), validCmd())

	require.NoError(t, err)
	require.Len(t, repo.Products, 1)
	assert.Equal(t, "classic-clog-rosa", repo.Products[0].Slug())
	assert.Equal(t, repo.Products[0].UID(), created.UID(), "devolve o produto criado")
}

func TestSave_RejectsDuplicateSlug(t *testing.T) {
	repo := &catalogtest.Repo{}
	h := commands.SaveHandler{Repository: repo}
	_, err0 := h.Handle(context.Background(), validCmd())
	require.NoError(t, err0)

	_, err := h.Handle(context.Background(), validCmd())

	var conflict httperr.ConflictError
	assert.True(t, errors.As(err, &conflict))
	assert.Len(t, repo.Products, 1)
}

func TestSave_InvalidInputDoesNotPersist(t *testing.T) {
	repo := &catalogtest.Repo{}
	cmd := validCmd()
	cmd.PriceCents = 0

	_, err := commands.SaveHandler{Repository: repo}.Handle(context.Background(), cmd)

	var req httperr.RequiredError
	assert.True(t, errors.As(err, &req))
	assert.Empty(t, repo.Products)
}
