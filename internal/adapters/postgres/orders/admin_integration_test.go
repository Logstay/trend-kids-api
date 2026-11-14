//go:build integration

package orders_test

import (
	"context"
	"errors"
	"sync"
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
	assert.Equal(t, 3, e.stock(t, rosa), "pagar não mexe no estoque")
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

func TestCancel_GivesStockBackWithLedgerEntry(t *testing.T) {
	e := newEnv(t)
	rosa := e.product(t, "Classic Clog Rosa", 18990, 5)
	charms := e.product(t, "Kit 5 Charms", 4990, 3)
	o := e.place(t, line{rosa, 2}, line{charms, 3})
	require.Equal(t, 3, e.stock(t, rosa))
	require.Equal(t, 0, e.stock(t, charms))
	admin := uuid.NewString()

	got, err := e.change(o.Code(), orders.StatusCancelled, admin)

	require.NoError(t, err)
	assert.Equal(t, orders.StatusCancelled, got.Status())
	assert.Equal(t, 5, e.stock(t, rosa), "devolveu os 2 pares")
	assert.Equal(t, 3, e.stock(t, charms), "devolveu os 3 kits")

	var mv struct {
		Delta    int    `db:"delta"`
		Reason   string `db:"reason"`
		OrderUID string `db:"order_uid"`
		Admin    string `db:"admin_uid"`
	}
	require.NoError(t, e.db.Get(&mv, `SELECT delta, reason, order_uid::text, admin_uid::text FROM stock_movements WHERE product_uid = $1 AND delta > 0 AND reason LIKE 'pedido cancelado%'`, rosa.UID()))
	assert.Equal(t, 2, mv.Delta)
	assert.Contains(t, mv.Reason, o.Code())
	assert.Equal(t, o.UID(), mv.OrderUID)
	assert.Equal(t, admin, mv.Admin)
}

func TestCancel_AlsoWorksFromPaidAndRestoresEvenIfProductIsNowInactive(t *testing.T) {
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

func TestCancel_AfterShippingIsRefusedAndStockStaysOut(t *testing.T) {
	e := newEnv(t)
	rosa := e.product(t, "Classic Clog Rosa", 18990, 5)
	o := e.place(t, line{rosa, 2})
	for _, to := range []orders.Status{orders.StatusPaid, orders.StatusShipped} {
		_, err := e.change(o.Code(), to, "")
		require.NoError(t, err)
	}

	_, err := e.change(o.Code(), orders.StatusCancelled, "")

	require.True(t, isConflict(err))
	assert.Equal(t, 3, e.stock(t, rosa))
}

// Dois cliques em "cancelar" (ou duas abas) não podem devolver o estoque duas vezes.
func TestCancel_ConcurrentCancelsRestoreStockExactlyOnce(t *testing.T) {
	e := newEnv(t)
	rosa := e.product(t, "Classic Clog Rosa", 18990, 5)
	o := e.place(t, line{rosa, 2})

	const clicks = 8
	var wg sync.WaitGroup
	errs := make(chan error, clicks)
	for i := 0; i < clicks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.change(o.Code(), orders.StatusCancelled, "")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	ok, refused := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			ok++
		case isConflict(err):
			refused++
		default:
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	assert.Equal(t, 1, ok)
	assert.Equal(t, clicks-1, refused)
	assert.Equal(t, 5, e.stock(t, rosa), "voltou ao estoque original, nem um a mais")
}

// Enviar e cancelar ao mesmo tempo: só um vence, e o estoque tem que bater com o vencedor.
func TestShipVersusCancelRace_StockMatchesTheWinner(t *testing.T) {
	e := newEnv(t)
	rosa := e.product(t, "Classic Clog Rosa", 18990, 1000)

	for i := 0; i < 15; i++ {
		o := e.place(t, line{rosa, 1})
		_, err := e.change(o.Code(), orders.StatusPaid, "")
		require.NoError(t, err)
		before := e.stock(t, rosa) // com este pedido ainda "fora" do estoque

		var wg sync.WaitGroup
		var shipErr, cancelErr error
		wg.Add(2)
		go func() { defer wg.Done(); _, shipErr = e.change(o.Code(), orders.StatusShipped, "") }()
		go func() { defer wg.Done(); _, cancelErr = e.change(o.Code(), orders.StatusCancelled, "") }()
		wg.Wait()

		require.True(t, (shipErr == nil) != (cancelErr == nil), "exatamente um vence (envio=%v cancelamento=%v)", shipErr, cancelErr)
		final, _ := e.orders.GetByCode(context.Background(), o.Code())
		if cancelErr == nil {
			assert.Equal(t, orders.StatusCancelled, final.Status())
			assert.Equal(t, before+1, e.stock(t, rosa), "cancelou: devolveu")
		} else {
			assert.Equal(t, orders.StatusShipped, final.Status())
			assert.Equal(t, before, e.stock(t, rosa), "enviou: estoque continua fora")
		}
	}
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
