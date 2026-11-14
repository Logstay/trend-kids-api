package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/application/catalog/commands"
	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	"github.com/Logstay/trend-kids-api/internal/ports/catalog/catalogtest"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seeded(t *testing.T) (*catalogtest.Repo, catalog.Product) {
	t.Helper()
	repo := &catalogtest.Repo{}
	p, err := commands.SaveHandler{Repository: repo}.Handle(context.Background(), validCmd())
	require.NoError(t, err)
	return repo, p
}

func updateCmd(uid string) ports.UpdateProduct {
	return ports.UpdateProduct{UID: uid, Name: "Classic Clog Rosa Pink", Category: "Classic", PriceCents: 15990, Active: true}
}

func TestUpdate_ChangesFieldsButKeepsSlugAndStock(t *testing.T) {
	repo, p := seeded(t)

	updated, err := commands.UpdateHandler{Repository: repo}.Handle(context.Background(), updateCmd(p.UID()))

	require.NoError(t, err)
	assert.Equal(t, "Classic Clog Rosa Pink", updated.Name())
	assert.Equal(t, int64(15990), updated.PriceCents())
	assert.Equal(t, "classic-clog-rosa", updated.Slug())
	assert.Equal(t, 3, updated.Stock())
	assert.Equal(t, "Classic Clog Rosa Pink", repo.Products[0].Name(), "foi persistido")
}

func TestUpdate_CanDeactivate(t *testing.T) {
	repo, p := seeded(t)
	cmd := updateCmd(p.UID())
	cmd.Active = false

	updated, err := commands.UpdateHandler{Repository: repo}.Handle(context.Background(), cmd)

	require.NoError(t, err)
	assert.False(t, updated.Active())
}

func TestUpdate_UnknownProductIsNotFound(t *testing.T) {
	_, err := commands.UpdateHandler{Repository: &catalogtest.Repo{}}.Handle(context.Background(), updateCmd("nao-existe"))

	var nf httperr.NotFoundError
	assert.True(t, errors.As(err, &nf))
}

func TestUpdate_InvalidInputChangesNothing(t *testing.T) {
	repo, p := seeded(t)
	cmd := updateCmd(p.UID())
	cmd.PriceCents = 0

	_, err := commands.UpdateHandler{Repository: repo}.Handle(context.Background(), cmd)

	var req httperr.RequiredError
	assert.True(t, errors.As(err, &req))
	assert.Equal(t, int64(18990), repo.Products[0].PriceCents())
}

func adjust(uid string, delta int, reason string) ports.AdjustStock {
	return ports.AdjustStock{UID: uid, Delta: delta, Reason: reason, AdminUID: "admin-1"}
}

func TestAdjustStock_AppliesDeltaAndRecordsWhoAndWhy(t *testing.T) {
	repo, p := seeded(t)

	stock, err := commands.AdjustStockHandler{Repository: repo}.Handle(context.Background(), adjust(p.UID(), -2, "venda no balcão"))

	require.NoError(t, err)
	assert.Equal(t, 1, stock)
	require.Len(t, repo.Movements, 1)
	assert.Equal(t, ports.StockMovement{ProductUID: p.UID(), Delta: -2, Reason: "venda no balcão", AdminUID: "admin-1"}, repo.Movements[0])
}

func TestAdjustStock_RejectsInvalidRequestsBeforeTouchingStock(t *testing.T) {
	repo, p := seeded(t)
	h := commands.AdjustStockHandler{Repository: repo}

	tests := map[string]ports.AdjustStock{
		"delta zero":    adjust(p.UID(), 0, "x"),
		"sem motivo":    adjust(p.UID(), 1, "   "),
		"motivo enorme": adjust(p.UID(), 1, string(make([]byte, ports.MaxReasonLen+1))),
	}
	for name, cmd := range tests {
		_, err := h.Handle(context.Background(), cmd)
		var req httperr.RequiredError
		assert.True(t, errors.As(err, &req), name)
	}
	assert.Empty(t, repo.Movements)
}

func TestAdjustStock_CannotGoNegative(t *testing.T) {
	repo, p := seeded(t)

	_, err := commands.AdjustStockHandler{Repository: repo}.Handle(context.Background(), adjust(p.UID(), -4, "venda"))

	var conflict httperr.ConflictError
	assert.True(t, errors.As(err, &conflict))
	assert.Equal(t, 3, repo.Products[0].Stock())
	assert.Empty(t, repo.Movements)
}
