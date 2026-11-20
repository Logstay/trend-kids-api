//go:build integration

package orders_test

import (
	"context"
	"errors"
	"os"
	"testing"

	pgcatalog "github.com/Logstay/trend-kids-api/internal/adapters/postgres/catalog"
	pgorders "github.com/Logstay/trend-kids-api/internal/adapters/postgres/orders"
	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type env struct {
	orders  *pgorders.Repository
	catalog *pgcatalog.Repository
	db      *sqlx.DB
}

func newEnv(t *testing.T) env {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL não definida")
	}
	db, err := sqlx.Connect("postgres", url)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`DROP TABLE IF EXISTS product_images, order_events, order_items, orders, stock_movements, products`)
	require.NoError(t, err)
	for _, f := range []string{"V1__create_products.sql", "V3__create_stock_movements.sql", "V4__create_orders.sql", "V5__create_order_events.sql", "V8__add_product_image.sql", "V9__create_product_images.sql"} {
		sqlText, err := os.ReadFile("../../../../config/migrations/sql/" + f)
		require.NoError(t, err)
		_, err = db.Exec(string(sqlText))
		require.NoError(t, err)
	}
	return env{orders: pgorders.NewRepository(db), catalog: pgcatalog.NewRepository(db), db: db}
}

func (e env) product(t *testing.T, name string, cents int64, stock int) catalog.Product {
	t.Helper()
	f := catalog.NewProductFactory()
	require.NoError(t, f.Name(name))
	require.NoError(t, f.Category("Classic"))
	require.NoError(t, f.PriceCents(cents))
	require.NoError(t, f.Stock(stock))
	p := f.Build()
	require.NoError(t, e.catalog.Save(context.Background(), p))
	return p
}

type line struct {
	p   catalog.Product
	qty int
}

func order(t *testing.T, lines ...line) orders.Order {
	t.Helper()
	f := orders.NewOrderFactory()
	require.NoError(t, f.Customer("Maria Silva", "(83) 99999-1234", "Manaíra"))
	for _, l := range lines {
		require.NoError(t, f.AddItem(l.p.UID(), l.p.Name(), l.p.PriceCents(), l.qty))
	}
	o, err := f.Build(orders.ShippingPolicy{FlatCents: 1500})
	require.NoError(t, err)
	return o
}

func (e env) stock(t *testing.T, p catalog.Product) int {
	t.Helper()
	got, err := e.catalog.GetByUID(context.Background(), p.UID())
	require.NoError(t, err)
	return got.Stock()
}

func (e env) count(t *testing.T, table string) int {
	t.Helper()
	var n int
	require.NoError(t, e.db.Get(&n, "SELECT count(*) FROM "+table))
	return n
}

func TestCreate_PersistsOrderWithoutTouchingStock(t *testing.T) {
	e := newEnv(t)
	rosa := e.product(t, "Classic Clog Rosa", 18990, 5)
	o := order(t, line{rosa, 2})
	movementsBefore := e.count(t, "stock_movements")

	require.NoError(t, e.orders.Create(context.Background(), o))

	assert.Equal(t, 5, e.stock(t, rosa), "o pedido não baixa estoque: quem dá a baixa é o vendedor")
	assert.Equal(t, movementsBefore, e.count(t, "stock_movements"))
}

func TestGetByCode_RoundTripKeepsSnapshots(t *testing.T) {
	e := newEnv(t)
	rosa := e.product(t, "Classic Clog Rosa", 18990, 5)
	o := order(t, line{rosa, 2})
	require.NoError(t, e.orders.Create(context.Background(), o))

	// O catálogo muda depois: o pedido não.
	_, err := e.db.Exec(`UPDATE products SET name = 'Outro nome', price_cents = 99999 WHERE uid = $1`, rosa.UID())
	require.NoError(t, err)
	got, err := e.orders.GetByCode(context.Background(), o.Code())

	require.NoError(t, err)
	assert.Equal(t, o.UID(), got.UID())
	assert.Equal(t, orders.StatusNew, got.Status())
	assert.Equal(t, "83999991234", got.CustomerPhone())
	assert.Equal(t, o.TotalCents(), got.TotalCents())
	require.Len(t, got.Items(), 1)
	assert.Equal(t, "Classic Clog Rosa", got.Items()[0].Name)
	assert.Equal(t, int64(18990), got.Items()[0].UnitPriceCents)
	assert.False(t, got.CreatedAt().IsZero())
}

func TestGetByCode_UnknownIsNotFound(t *testing.T) {
	e := newEnv(t)
	_, err := e.orders.GetByCode(context.Background(), "TK22222222")
	var nf httperr.NotFoundError
	assert.True(t, errors.As(err, &nf))
}

func TestCreate_DuplicateCodeReturnsErrCodeTaken(t *testing.T) {
	e := newEnv(t)
	rosa := e.product(t, "Classic Clog Rosa", 18990, 9)
	first := order(t, line{rosa, 1})
	require.NoError(t, e.orders.Create(context.Background(), first))

	clone := orders.Restore(orders.Snapshot{
		UID: "11111111-1111-1111-1111-111111111111", Code: first.Code(), CustomerName: "João", Phone: "83999991234",
		Neighborhood: "Bessa", Status: orders.StatusNew, Items: first.Items(),
		SubtotalCents: first.SubtotalCents(), ShippingCents: first.ShippingCents(), TotalCents: first.TotalCents(),
	})
	err := e.orders.Create(context.Background(), clone)

	assert.ErrorIs(t, err, ports.ErrCodeTaken)
	assert.Equal(t, 9, e.stock(t, rosa), "nenhum pedido baixa estoque")
}
