package http_test

import (
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Logstay/trend-kids-api/internal/adapters/http"
	ordercmd "github.com/Logstay/trend-kids-api/internal/application/orders/commands"
	orderq "github.com/Logstay/trend-kids-api/internal/application/orders/queries"
	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	"github.com/Logstay/trend-kids-api/internal/ports/catalog/catalogtest"
	"github.com/Logstay/trend-kids-api/internal/ports/orders/orderstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type store struct {
	handler nethttp.Handler
	orders  *orderstest.Repo
	rosa    catalog.Product // R$ 189,90, estoque 5
	charms  catalog.Product // R$ 49,90, estoque 1
}

func newStore(t *testing.T, limit int) store {
	t.Helper()
	mk := func(name string, cents int64, stock int) catalog.Product {
		f := catalog.NewProductFactory()
		require.NoError(t, f.Name(name))
		require.NoError(t, f.Category("Classic"))
		require.NoError(t, f.PriceCents(cents))
		require.NoError(t, f.Stock(stock))
		return f.Build()
	}
	rosa, charms := mk("Classic Clog Rosa", 18990, 5), mk("Kit 5 Charms", 4990, 1)
	cat, ord := &catalogtest.Repo{Products: []catalog.Product{rosa, charms}}, &orderstest.Repo{}

	h := http.NewRouter(http.Deps{
		AllowedOrigins: []string{"http://localhost:3000"},
		OrderLimit:     limit,
		OrderWindow:    time.Minute,
		Orders: &http.Orders{
			Place:  ordercmd.PlaceHandler{Orders: ord, Products: cat, Shipping: orders.ShippingPolicy{FlatCents: 1500}},
			Lookup: orderq.LookupHandler{Orders: ord},
		},
	})
	return store{handler: h, orders: ord, rosa: rosa, charms: charms}
}

func orderBody(items string) string {
	return `{"customer":{"name":"Maria Silva","phone":"(83) 99999-1234","neighborhood":"Manaíra"},"items":` + items + `}`
}

func (s store) post(url, body string, mods ...func(*nethttp.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.1:1234"
	for _, m := range mods {
		m(req)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

func TestPlaceOrder_Returns201WithTotalsComputedByTheServer(t *testing.T) {
	s := newStore(t, 100)
	// O cliente tenta impor preço e total: nada disso pode valer.
	body := orderBody(`[{"product_id":"` + s.rosa.UID() + `","quantity":2,"unit_price_cents":1,"total_cents":1}]`)

	rec := s.post("/v1/orders", body)

	require.Equal(t, 201, rec.Code, rec.Body.String())
	var out struct {
		Code          string `json:"code"`
		Status        string `json:"status"`
		SubtotalCents int64  `json:"subtotal_cents"`
		ShippingCents int64  `json:"shipping_cents"`
		TotalCents    int64  `json:"total_cents"`
		Items         []struct {
			Name           string `json:"name"`
			UnitPriceCents int64  `json:"unit_price_cents"`
			Quantity       int    `json:"quantity"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Regexp(t, `^TK[2-9A-HJKMNP-Z]{8}$`, out.Code)
	assert.Equal(t, "new", out.Status)
	assert.Equal(t, int64(2*18990), out.SubtotalCents)
	assert.Equal(t, int64(1500), out.ShippingCents)
	assert.Equal(t, int64(2*18990+1500), out.TotalCents)
	require.Len(t, out.Items, 1)
	assert.Equal(t, int64(18990), out.Items[0].UnitPriceCents)
	assert.Equal(t, "Classic Clog Rosa", out.Items[0].Name)
	assert.NotContains(t, rec.Body.String(), "99999", "o telefone não volta na resposta")
	assert.Len(t, s.orders.Orders, 1)
}

func TestPlaceOrder_ValidationErrors(t *testing.T) {
	s := newStore(t, 100)
	item := func(q string) string { return `[{"product_id":"` + s.rosa.UID() + `","quantity":` + q + `}]` }
	badPhone := `{"customer":{"name":"Maria","phone":"123","neighborhood":"Manaíra"},"items":` + item("1") + `}`

	for name, body := range map[string]string{
		"sem itens":          orderBody(`[]`),
		"itens ausentes":     `{"customer":{"name":"Maria Silva","phone":"83999991234","neighborhood":"Manaíra"}}`,
		"quantidade zero":    orderBody(item("0")),
		"quantidade 21":      orderBody(item("21")),
		"quantidade decimal": orderBody(item("1.5")),
		"telefone inválido":  badPhone,
		"json quebrado":      `{`,
	} {
		assert.Equal(t, 400, s.post("/v1/orders", body).Code, name)
	}
	assert.Empty(t, s.orders.Orders)
}

func TestPlaceOrder_UnknownProduct404AndInsufficientStock409(t *testing.T) {
	s := newStore(t, 100)

	unknown := s.post("/v1/orders", orderBody(`[{"product_id":"nao-existe","quantity":1}]`))
	short := s.post("/v1/orders", orderBody(`[{"product_id":"`+s.charms.UID()+`","quantity":2}]`))

	assert.Equal(t, 404, unknown.Code)
	assert.Equal(t, 409, short.Code)
	assert.Contains(t, short.Body.String(), "Kit 5 Charms")
	assert.Empty(t, s.orders.Orders)
}

func TestPlaceOrder_RequiresJSONContentType(t *testing.T) {
	s := newStore(t, 100)
	rec := s.post("/v1/orders", orderBody(`[{"product_id":"`+s.rosa.UID()+`","quantity":1}]`),
		func(r *nethttp.Request) { r.Header.Set("Content-Type", "text/plain") })
	assert.Equal(t, 400, rec.Code)
}

func TestPlaceOrder_IsRateLimitedPerClient(t *testing.T) {
	s := newStore(t, 2)
	body := orderBody(`[{"product_id":"nao-existe","quantity":1}]`)

	s.post("/v1/orders", body)
	s.post("/v1/orders", body)
	blocked := s.post("/v1/orders", body)
	other := s.post("/v1/orders", body, func(r *nethttp.Request) { r.RemoteAddr = "10.0.0.2:1" })

	assert.Equal(t, 429, blocked.Code)
	assert.Equal(t, 404, other.Code, "outro IP tem a própria cota")
}

func TestLookup_ReturnsOrderForCodeAndPhone(t *testing.T) {
	s := newStore(t, 100)
	created := s.post("/v1/orders", orderBody(`[{"product_id":"`+s.rosa.UID()+`","quantity":1}]`))
	var c struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &c))

	rec := s.post("/v1/orders/lookup", `{"code":"`+c.Code+`","phone":"83 99999-1234"}`)

	require.Equal(t, 200, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"`+c.Code+`"`)
	assert.Contains(t, rec.Body.String(), `"status":"new"`)
}

func TestLookup_WrongPhoneAndUnknownCodeAreIndistinguishable(t *testing.T) {
	s := newStore(t, 100)
	created := s.post("/v1/orders", orderBody(`[{"product_id":"`+s.rosa.UID()+`","quantity":1}]`))
	var c struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &c))

	wrongPhone := s.post("/v1/orders/lookup", `{"code":"`+c.Code+`","phone":"83988887777"}`)
	unknown := s.post("/v1/orders/lookup", `{"code":"TK22222222","phone":"83999991234"}`)

	assert.Equal(t, 404, wrongPhone.Code)
	assert.Equal(t, 404, unknown.Code)
	assert.Equal(t, wrongPhone.Body.String(), unknown.Body.String())
}

func TestLookup_RequiresBothFields(t *testing.T) {
	s := newStore(t, 100)
	assert.Equal(t, 400, s.post("/v1/orders/lookup", `{"code":"TK22222222"}`).Code)
	assert.Equal(t, 400, s.post("/v1/orders/lookup", `{"phone":"83999991234"}`).Code)
}
