package commands_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/application/orders/commands"
	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	catalogports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	"github.com/Logstay/trend-kids-api/internal/ports/catalog/catalogtest"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	"github.com/Logstay/trend-kids-api/internal/ports/orders/orderstest"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type shop struct {
	catalog *catalogtest.Repo
	orders  *orderstest.Repo
	place   commands.PlaceHandler
	rosa    catalog.Product // R$ 189,90, estoque 5
	charms  catalog.Product // R$ 49,90,  estoque 2
}

func product(t *testing.T, name string, cents int64, stock int, active bool) catalog.Product {
	t.Helper()
	f := catalog.NewProductFactory()
	require.NoError(t, f.Name(name))
	require.NoError(t, f.Category("Classic"))
	require.NoError(t, f.PriceCents(cents))
	require.NoError(t, f.Stock(stock))
	f.Active(active)
	return f.Build()
}

func newShop(t *testing.T) shop {
	t.Helper()
	rosa, charms := product(t, "Classic Clog Rosa", 18990, 5, true), product(t, "Kit 5 Charms", 4990, 2, true)
	cat, ord := &catalogtest.Repo{Products: []catalog.Product{rosa, charms}}, &orderstest.Repo{}
	_ = catalogports.DefaultLimit
	return shop{
		catalog: cat, orders: ord, rosa: rosa, charms: charms,
		place: commands.PlaceHandler{Orders: ord, Products: cat, Shipping: orders.ShippingPolicy{FlatCents: 1500, FreeAboveCents: 50000}},
	}
}

func (s shop) cmd(items ...ports.PlaceItem) ports.PlaceOrder {
	return ports.PlaceOrder{CustomerName: "Maria Silva", Phone: "(83) 99999-1234", Neighborhood: "Manaíra", Items: items}
}

func TestPlace_PricesAndTotalsComeFromTheCatalog(t *testing.T) {
	s := newShop(t)

	o, err := s.place.Handle(context.Background(), s.cmd(
		ports.PlaceItem{ProductUID: s.rosa.UID(), Quantity: 2},
		ports.PlaceItem{ProductUID: s.charms.UID(), Quantity: 1},
	))

	require.NoError(t, err)
	assert.Equal(t, int64(2*18990+4990), o.SubtotalCents())
	assert.Equal(t, int64(1500), o.ShippingCents(), "abaixo do mínimo para frete grátis")
	assert.Equal(t, o.SubtotalCents()+1500, o.TotalCents())
	require.Len(t, s.orders.Orders, 1)
	assert.Equal(t, "Classic Clog Rosa", o.Items()[0].Name, "snapshot do nome")
	assert.Equal(t, int64(18990), o.Items()[0].UnitPriceCents, "snapshot do preço")
	assert.Equal(t, "83999991234", o.CustomerPhone())
}

func TestPlace_FreeShippingAboveThreshold(t *testing.T) {
	s := newShop(t)
	o, err := s.place.Handle(context.Background(), s.cmd(ports.PlaceItem{ProductUID: s.rosa.UID(), Quantity: 3})) // 569,70
	require.NoError(t, err)
	assert.Equal(t, int64(0), o.ShippingCents())
}

func TestPlace_UnknownProductIsNotFoundAndNothingIsSaved(t *testing.T) {
	s := newShop(t)

	_, err := s.place.Handle(context.Background(), s.cmd(ports.PlaceItem{ProductUID: "nao-existe", Quantity: 1}))

	var nf httperr.NotFoundError
	assert.True(t, errors.As(err, &nf))
	assert.Empty(t, s.orders.Orders)
}

func TestPlace_InactiveProductIsUnavailable(t *testing.T) {
	s := newShop(t)
	hidden := product(t, "Oculto", 1000, 5, false)
	s.catalog.Products = append(s.catalog.Products, hidden)

	_, err := s.place.Handle(context.Background(), s.cmd(ports.PlaceItem{ProductUID: hidden.UID(), Quantity: 1}))

	var conflict httperr.ConflictError
	require.True(t, errors.As(err, &conflict))
	assert.Contains(t, err.Error(), "Oculto")
	assert.Empty(t, s.orders.Orders)
}

func TestPlace_InsufficientStockNamesTheProduct(t *testing.T) {
	s := newShop(t)

	_, err := s.place.Handle(context.Background(), s.cmd(ports.PlaceItem{ProductUID: s.charms.UID(), Quantity: 3}))

	var conflict httperr.ConflictError
	require.True(t, errors.As(err, &conflict))
	assert.Contains(t, err.Error(), "Kit 5 Charms")
	assert.Empty(t, s.orders.Orders)
}

func TestPlace_DuplicateLinesAreMergedBeforeCheckingStock(t *testing.T) {
	s := newShop(t) // charms: estoque 2

	_, err := s.place.Handle(context.Background(), s.cmd(
		ports.PlaceItem{ProductUID: s.charms.UID(), Quantity: 2},
		ports.PlaceItem{ProductUID: s.charms.UID(), Quantity: 1},
	))

	var conflict httperr.ConflictError
	assert.True(t, errors.As(err, &conflict), "2+1 passa do estoque 2, mesmo que cada linha caiba")
}

func TestPlace_InvalidCustomerOrEmptyCartIsRejected(t *testing.T) {
	s := newShop(t)
	var req httperr.RequiredError

	bad := s.cmd(ports.PlaceItem{ProductUID: s.rosa.UID(), Quantity: 1})
	bad.Phone = "123"
	_, err := s.place.Handle(context.Background(), bad)
	assert.True(t, errors.As(err, &req), "telefone inválido")

	_, err = s.place.Handle(context.Background(), s.cmd())
	assert.True(t, errors.As(err, &req), "carrinho vazio")
	assert.Empty(t, s.orders.Orders)
}

func TestPlace_RetriesWithNewCodeOnCollision(t *testing.T) {
	s := newShop(t)
	s.orders.TakenFor = 2

	o, err := s.place.Handle(context.Background(), s.cmd(ports.PlaceItem{ProductUID: s.rosa.UID(), Quantity: 1}))

	require.NoError(t, err)
	assert.Equal(t, 3, s.orders.Calls)
	assert.Len(t, s.orders.Orders, 1)
	assert.NotEmpty(t, o.Code())
}

func TestPlace_GivesUpAfterRepeatedCollisions(t *testing.T) {
	s := newShop(t)
	s.orders.TakenFor = 10

	_, err := s.place.Handle(context.Background(), s.cmd(ports.PlaceItem{ProductUID: s.rosa.UID(), Quantity: 1}))

	assert.Error(t, err)
	assert.Equal(t, 3, s.orders.Calls)
}

func TestPlace_PropagatesRepositoryConflict(t *testing.T) {
	s := newShop(t)
	s.orders.CreateErr = httperr.ConflictError("estoque insuficiente: Classic Clog Rosa")

	_, err := s.place.Handle(context.Background(), s.cmd(ports.PlaceItem{ProductUID: s.rosa.UID(), Quantity: 1}))

	var conflict httperr.ConflictError
	assert.True(t, errors.As(err, &conflict))
}
