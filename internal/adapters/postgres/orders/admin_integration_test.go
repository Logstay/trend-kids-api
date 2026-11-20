//go:build integration

package orders_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (e env) change(code string, to orders.Status, admin string) (orders.Order, error) {
	return e.orders.UpdateStatus(context.Background(), ports.ChangeStatus{Code: code, To: to, AdminUID: admin})
}

func (e env) place(t *testing.T, lines ...line) orders.Order {
	t.Helper()
	o := order(t, lines...)
	require.NoError(t, e.orders.Create(context.Background(), o))
	return o
}

func isConflict(err error) bool {
	var c httperr.ConflictError
	return errors.As(err, &c)
}

func TestCreate_RecordsCreationEvent(t *testing.T) {
	e := newEnv(t)
	o := e.place(t, line{e.product(t, "Classic Clog Rosa", 18990, 5), 1})

	events, err := e.orders.Events(context.Background(), o.UID())

	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, orders.Status(""), events[0].From, "criação não tem status anterior")
	assert.Equal(t, orders.StatusNew, events[0].To)
	assert.Empty(t, events[0].AdminUID)
	assert.False(t, events[0].At.IsZero())
}

func TestUpdateStatus_AdvancesRecordsEventAndKeepsStock(t *testing.T) {
	e := newEnv(t)
	rosa := e.product(t, "Classic Clog Rosa", 18990, 5)
	o := e.place(t, line{rosa, 2})
	admin := uuid.NewString()

	got, err := e.change(o.Code(), orders.StatusPaid, admin)

	require.NoError(t, err)
	assert.Equal(t, orders.StatusPaid, got.Status())
	assert.Equal(t, 5, e.stock(t, rosa), "pagar não mexe no estoque")
	events, _ := e.orders.Events(context.Background(), o.UID())
	require.Len(t, events, 2)
	assert.Equal(t, orders.StatusNew, events[1].From)
	assert.Equal(t, orders.StatusPaid, events[1].To)
	assert.Equal(t, admin, events[1].AdminUID)
}

func TestUpdateStatus_ForbiddenTransitionChangesNothing(t *testing.T) {
	e := newEnv(t)
	o := e.place(t, line{e.product(t, "Classic Clog Rosa", 18990, 5), 1})

	_, err := e.change(o.Code(), orders.StatusShipped, "") // new → shipped pula o pagamento

	require.True(t, isConflict(err))
	got, _ := e.orders.GetByCode(context.Background(), o.Code())
	assert.Equal(t, orders.StatusNew, got.Status())
	events, _ := e.orders.Events(context.Background(), o.UID())
	assert.Len(t, events, 1, "nenhum evento novo")
}

func TestUpdateStatus_UnknownCodeIsNotFound(t *testing.T) {
	e := newEnv(t)
	_, err := e.change("TK22222222", orders.StatusPaid, "")
	var nf httperr.NotFoundError
	assert.True(t, errors.As(err, &nf))
}

func TestCancel_DoesNotTouchStock(t *testing.T) {
	e := newEnv(t)
	rosa := e.product(t, "Classic Clog Rosa", 18990, 5)
	charms := e.product(t, "Kit 5 Charms", 4990, 3)
	o := e.place(t, line{rosa, 2}, line{charms, 3})
	movementsBefore := e.count(t, "stock_movements")

	got, err := e.change(o.Code(), orders.StatusCancelled, uuid.NewString())

	require.NoError(t, err)
	assert.Equal(t, orders.StatusCancelled, got.Status())
	assert.Equal(t, 5, e.stock(t, rosa), "o estoque nunca foi baixado, então não há o que devolver")
	assert.Equal(t, 3, e.stock(t, charms))
	assert.Equal(t, movementsBefore, e.count(t, "stock_movements"))
}

func TestCancel_FromPaidWorksEvenIfProductIsNowInactive(t *testing.T) {
	e := newEnv(t)
	rosa := e.product(t, "Classic Clog Rosa", 18990, 5)
	o := e.place(t, line{rosa, 2})
	_, err := e.change(o.Code(), orders.StatusPaid, "")
	require.NoError(t, err)
	_, err = e.db.Exec(`UPDATE products SET active = false WHERE uid = $1`, rosa.UID())
	require.NoError(t, err)

	_, err = e.change(o.Code(), orders.StatusCancelled, "")

	require.NoError(t, err)
	assert.Equal(t, 5, e.stock(t, rosa))
}

func TestCancel_AfterShippingIsRefusedAndStockUnchanged(t *testing.T) {
	e := newEnv(t)
	rosa := e.product(t, "Classic Clog Rosa", 18990, 5)
	o := e.place(t, line{rosa, 2})
	for _, to := range []orders.Status{orders.StatusPaid, orders.StatusShipped} {
		_, err := e.change(o.Code(), to, "")
		require.NoError(t, err)
	}

	_, err := e.change(o.Code(), orders.StatusCancelled, "")

	require.True(t, isConflict(err))
	assert.Equal(t, 5, e.stock(t, rosa))
}

func TestList_FiltersByStatusNewestFirstWithItemsAndPaging(t *testing.T) {
	e := newEnv(t)
	rosa := e.product(t, "Classic Clog Rosa", 18990, 50)
	first := e.place(t, line{rosa, 1})
	second := e.place(t, line{rosa, 2})
	third := e.place(t, line{rosa, 3})
	_, err := e.change(second.Code(), orders.StatusPaid, "")
	require.NoError(t, err)
	ctx := context.Background()

	all, err := e.orders.List(ctx, ports.ListFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, third.Code(), all[0].Code(), "mais novo primeiro")
	assert.Equal(t, first.Code(), all[2].Code())
	require.Len(t, all[0].Items(), 1)
	assert.Equal(t, 3, all[0].Items()[0].Quantity, "itens vêm junto")

	paid, err := e.orders.List(ctx, ports.ListFilter{Status: "paid", Limit: 10})
	require.NoError(t, err)
	require.Len(t, paid, 1)
	assert.Equal(t, second.Code(), paid[0].Code())

	page, err := e.orders.List(ctx, ports.ListFilter{Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, second.Code(), page[0].Code())
}

func TestList_EmptyIsNotNil(t *testing.T) {
	e := newEnv(t)
	got, err := e.orders.List(context.Background(), ports.ListFilter{Limit: 10})
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}
