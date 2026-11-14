package queries_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/application/orders/queries"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	"github.com/Logstay/trend-kids-api/internal/ports/orders/orderstest"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminList_DefaultsAndLimits(t *testing.T) {
	for name, tc := range map[string]struct{ in, want int }{
		"zero vira padrão": {0, ports.DefaultLimit}, "negativo vira padrão": {-3, ports.DefaultLimit},
		"acima do máximo é cortado": {1000, ports.MaxLimit}, "válido é mantido": {7, 7},
	} {
		repo := &orderstest.Repo{}
		_, err := queries.AdminListHandler{Orders: repo}.Handle(context.Background(), ports.ListFilter{Limit: tc.in})
		require.NoError(t, err, name)
		assert.Equal(t, tc.want, repo.LastFilter.Limit, name)
	}
}

func TestAdminList_NegativeOffsetBecomesZeroAndStatusIsPassedThrough(t *testing.T) {
	repo := &orderstest.Repo{}
	_, err := queries.AdminListHandler{Orders: repo}.Handle(context.Background(), ports.ListFilter{Offset: -5, Status: "paid"})
	require.NoError(t, err)
	assert.Equal(t, 0, repo.LastFilter.Offset)
	assert.Equal(t, "paid", repo.LastFilter.Status)
}

func TestAdminList_UnknownStatusFilterIsBadRequest(t *testing.T) {
	_, err := queries.AdminListHandler{Orders: &orderstest.Repo{}}.Handle(context.Background(), ports.ListFilter{Status: "pago"})
	var bad httperr.BadRequestError
	assert.True(t, errors.As(err, &bad))
}

func TestAdminGet_ReturnsOrderWithHistory(t *testing.T) {
	h, o := seeded(t) // do lookup_test.go: pedido novo no repositório em memória
	repo := h.Orders.(*orderstest.Repo)
	_, err := repo.UpdateStatus(context.Background(), ports.ChangeStatus{Code: o.Code(), To: "paid", AdminUID: "admin-1"})
	require.NoError(t, err)

	view, err := queries.AdminGetHandler{Orders: repo}.Handle(context.Background(), "  "+lower(o.Code())+" ")

	require.NoError(t, err)
	assert.Equal(t, o.UID(), view.Order.UID())
	require.Len(t, view.Events, 1)
	assert.Equal(t, "admin-1", view.Events[0].AdminUID)
}

func TestAdminGet_UnknownCodeIsNotFound(t *testing.T) {
	_, err := queries.AdminGetHandler{Orders: &orderstest.Repo{}}.Handle(context.Background(), "TK22222222")
	var nf httperr.NotFoundError
	assert.True(t, errors.As(err, &nf))
}
