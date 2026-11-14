package http_test

import (
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/adapters/http"
	"github.com/Logstay/trend-kids-api/internal/application/catalog/queries"
	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	"github.com/Logstay/trend-kids-api/internal/ports/catalog/catalogtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newServer(t *testing.T, products ...catalog.Product) (nethttp.Handler, *catalogtest.Repo) {
	t.Helper()
	repo := &catalogtest.Repo{Products: products}
	return http.NewRouter(http.Deps{
		List:           queries.ListHandler{Repository: repo},
		Get:            queries.GetHandler{Repository: repo},
		AllowedOrigins: []string{"http://localhost:3000"},
	}), repo
}

func product(t *testing.T, name, category string, cents int64) catalog.Product {
	t.Helper()
	f := catalog.NewProductFactory()
	require.NoError(t, f.Name(name))
	require.NoError(t, f.Category(category))
	require.NoError(t, f.PriceCents(cents))
	require.NoError(t, f.Stock(2))
	return f.Build()
}

func do(h nethttp.Handler, method, url string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealth(t *testing.T) {
	h, _ := newServer(t)
	assert.Equal(t, 200, do(h, "GET", "/health").Code)
}

func TestListProducts_ReturnsItemsWithPriceInCents(t *testing.T) {
	h, _ := newServer(t, product(t, "Classic Clog Rosa", "Classic", 18990))

	rec := do(h, "GET", "/v1/products")

	require.Equal(t, 200, rec.Code)
	var body struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Equal(t, "classic-clog-rosa", body.Items[0]["slug"])
	assert.Equal(t, float64(18990), body.Items[0]["price_cents"])
	assert.Equal(t, "Classic", body.Items[0]["category"])
}

func TestListProducts_EmptyIsArrayNotNull(t *testing.T) {
	h, _ := newServer(t)
	assert.JSONEq(t, `{"items":[]}`, do(h, "GET", "/v1/products").Body.String())
}

func TestListProducts_ParsesQueryParams(t *testing.T) {
	h, repo := newServer(t)

	do(h, "GET", "/v1/products?category=Com+luz&q=dino&sort=price_asc&limit=5&offset=10")

	assert.Equal(t, ports.Filter{Category: "Com luz", Query: "dino", Sort: ports.SortPriceAsc, Limit: 5, Offset: 10}, repo.LastFilter)
}

func TestListProducts_InvalidParamsAreBadRequest(t *testing.T) {
	h, _ := newServer(t)
	for _, url := range []string{"/v1/products?limit=abc", "/v1/products?offset=-1", "/v1/products?sort=random"} {
		assert.Equal(t, 400, do(h, "GET", url).Code, url)
	}
}

func TestGetProduct_BySlug(t *testing.T) {
	h, _ := newServer(t, product(t, "Classic Clog Rosa", "Classic", 18990))

	rec := do(h, "GET", "/v1/products/classic-clog-rosa")

	require.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), `"name":"Classic Clog Rosa"`)
}

func TestGetProduct_NotFoundHasJSONError(t *testing.T) {
	h, _ := newServer(t)

	rec := do(h, "GET", "/v1/products/nao-existe")

	assert.Equal(t, 404, rec.Code)
	assert.JSONEq(t, `{"code":404,"error":"produto não encontrado"}`, rec.Body.String())
}

func TestCORS_AllowsConfiguredOriginOnly(t *testing.T) {
	h, _ := newServer(t)

	ok := do(h, "GET", "/v1/products", "Origin", "http://localhost:3000")
	bad := do(h, "GET", "/v1/products", "Origin", "https://evil.example")

	assert.Equal(t, "http://localhost:3000", ok.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, bad.Header().Get("Access-Control-Allow-Origin"))
}
