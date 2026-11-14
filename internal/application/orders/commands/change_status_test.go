package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/application/orders/commands"
	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	"github.com/Logstay/trend-kids-api/internal/ports/orders/orderstest"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pending(t *testing.T) (*orderstest.Repo, orders.Order) {
	t.Helper()
	f := orders.NewOrderFactory()
	require.NoError(t, f.Customer("Maria Silva", "83999991234", "Manaíra"))
	require.NoError(t, f.AddItem("p1", "Classic Clog Rosa", 18990, 1))
	o, err := f.Build(orders.ShippingPolicy{})
	require.NoError(t, err)
	return &orderstest.Repo{Orders: []orders.Order{o}}, o
}

func TestChangeStatus_AdvancesAndRecordsWho(t *testing.T) {
	repo, o := pending(t)

	got, err := commands.ChangeStatusHandler{Orders: repo}.Handle(context.Background(),
		ports.ChangeStatusCommand{Code: o.Code(), Status: "paid", AdminUID: "admin-1"})

	require.NoError(t, err)
	assert.Equal(t, orders.StatusPaid, got.Status())
	require.Len(t, repo.EventLog[o.UID()], 1)
	assert.Equal(t, "admin-1", repo.EventLog[o.UID()][0].AdminUID)
}

func TestChangeStatus_CodeIsNormalized(t *testing.T) {
	repo, o := pending(t)

	_, err := commands.ChangeStatusHandler{Orders: repo}.Handle(context.Background(),
		ports.ChangeStatusCommand{Code: "  " + lowerCode(o.Code()) + " ", Status: "paid"})

	assert.NoError(t, err)
}

func lowerCode(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func TestChangeStatus_UnknownStatusNeverReachesTheRepository(t *testing.T) {
	repo, o := pending(t)

	for _, bad := range []string{"", "PAID", "pago", "refunded"} {
		_, err := commands.ChangeStatusHandler{Orders: repo}.Handle(context.Background(), ports.ChangeStatusCommand{Code: o.Code(), Status: bad})
		var req httperr.RequiredError
		assert.True(t, errors.As(err, &req), bad)
	}
	assert.Empty(t, repo.Changes)
}

func TestChangeStatus_ForbiddenTransitionIsConflict(t *testing.T) {
	repo, o := pending(t)

	for _, to := range []string{"shipped", "delivered", "new"} {
		_, err := commands.ChangeStatusHandler{Orders: repo}.Handle(context.Background(), ports.ChangeStatusCommand{Code: o.Code(), Status: to})
		var conflict httperr.ConflictError
		assert.True(t, errors.As(err, &conflict), "new → "+to)
	}
	assert.Equal(t, orders.StatusNew, repo.Orders[0].Status())
}

func TestChangeStatus_UnknownOrderIsNotFound(t *testing.T) {
	_, err := commands.ChangeStatusHandler{Orders: &orderstest.Repo{}}.Handle(context.Background(), ports.ChangeStatusCommand{Code: "TK22222222", Status: "paid"})
	var nf httperr.NotFoundError
	assert.True(t, errors.As(err, &nf))
}
