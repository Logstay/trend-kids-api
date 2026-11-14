package http_test

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Logstay/trend-kids-api/internal/adapters/http"
	"github.com/Logstay/trend-kids-api/internal/adapters/security"
	catalogcmd "github.com/Logstay/trend-kids-api/internal/application/catalog/commands"
	catalogq "github.com/Logstay/trend-kids-api/internal/application/catalog/queries"
	identitycmd "github.com/Logstay/trend-kids-api/internal/application/identity/commands"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	"github.com/Logstay/trend-kids-api/internal/ports/catalog/catalogtest"
	identityports "github.com/Logstay/trend-kids-api/internal/ports/identity"
	"github.com/Logstay/trend-kids-api/internal/ports/identity/identitytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type shop struct {
	handler nethttp.Handler
	cookie  *nethttp.Cookie
	repo    *catalogtest.Repo
	adminID string
}

func newShop(t *testing.T) shop {
	t.Helper()
	admins := &identitytest.Repo{}
	hasher := security.NewArgon2Hasher()
	tokens, err := security.NewTokenService("0123456789abcdef0123456789abcdef", time.Hour, time.Now)
	require.NoError(t, err)
	require.NoError(t, identitycmd.CreateAdminHandler{Repository: admins, Hasher: hasher}.
		Handle(context.Background(), identityports.CreateAdminCommand{Email: "dono@trendkids.com.br", Password: adminPassword}))
	login := identitycmd.LoginHandler{Repository: admins, Hasher: hasher, Tokens: tokens}

	repo := &catalogtest.Repo{}
	h := http.NewRouter(http.Deps{
		AllowedOrigins: []string{"http://localhost:3000"},
		Login:          login,
		Tokens:         tokens,
		LoginLimit:     100,
		Catalog: &http.AdminCatalog{
			List:        catalogq.ListHandler{Repository: repo},
			Create:      catalogcmd.SaveHandler{Repository: repo},
			Update:      catalogcmd.UpdateHandler{Repository: repo},
			AdjustStock: catalogcmd.AdjustStockHandler{Repository: repo},
		},
	})
	rec := post(h, "/v1/admin/login", loginBody("dono@trendkids.com.br", adminPassword))
	require.Equal(t, 204, rec.Code)
	return shop{handler: h, cookie: sessionCookie(rec), repo: repo, adminID: admins.Admins[0].UID()}
}

func (s shop) do(method, url, body string, withSession bool, mods ...func(*nethttp.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if withSession {
		req.AddCookie(s.cookie)
	}
	for _, m := range mods {
		m(req)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec
}

const newProduct = `{"name":"Classic Clog Rosa","category":"Classic","description":"d","price_cents":18990,"stock":3}`

func (s shop) create(t *testing.T) string {
	t.Helper()
	rec := s.do("POST", "/v1/admin/products", newProduct, true)
	require.Equal(t, 201, rec.Code, rec.Body.String())
	var body struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.ID
}

func TestAdminCatalog_EveryRouteRequiresSession(t *testing.T) {
	s := newShop(t)
	for _, r := range [][2]string{
		{"GET", "/v1/admin/products"},
		{"POST", "/v1/admin/products"},
		{"PUT", "/v1/admin/products/qualquer"},
		{"PATCH", "/v1/admin/products/qualquer/stock"},
	} {
		assert.Equal(t, 401, s.do(r[0], r[1], newProduct, false).Code, r[0]+" "+r[1])
	}
	assert.Empty(t, s.repo.Products, "nada foi criado sem sessão")
}

func TestAdminCatalog_CreateReturns201WithProduct(t *testing.T) {
	s := newShop(t)

	rec := s.do("POST", "/v1/admin/products", newProduct, true)

	require.Equal(t, 201, rec.Code)
	assert.Contains(t, rec.Body.String(), `"slug":"classic-clog-rosa"`)
	assert.Contains(t, rec.Body.String(), `"price_cents":18990`)
	assert.Len(t, s.repo.Products, 1)
}

func TestAdminCatalog_CreateRejectsInvalidAndDuplicate(t *testing.T) {
	s := newShop(t)
	assert.Equal(t, 400, s.do("POST", "/v1/admin/products", `{"name":"X","category":"Classic","price_cents":0}`, true).Code, "preço zero")
	assert.Equal(t, 400, s.do("POST", "/v1/admin/products", `{"name":"X","category":"Classic","price_cents":10.5}`, true).Code, "centavos fracionados")
	assert.Equal(t, 400, s.do("POST", "/v1/admin/products", `{"name":"X","category":"Chinelo","price_cents":100}`, true).Code, "categoria")
	s.create(t)
	assert.Equal(t, 409, s.do("POST", "/v1/admin/products", newProduct, true).Code, "nome repetido")
}

func TestAdminCatalog_ListIncludesInactive(t *testing.T) {
	s := newShop(t)

	s.do("GET", "/v1/admin/products?limit=5", "", true)

	assert.True(t, s.repo.LastFilter.IncludeInactive)
	assert.Equal(t, 5, s.repo.LastFilter.Limit)
}

func TestAdminCatalog_UpdateKeepsStockEvenIfBodyTriesToChangeIt(t *testing.T) {
	s := newShop(t)
	id := s.create(t)

	rec := s.do("PUT", "/v1/admin/products/"+id,
		`{"name":"Classic Clog Rosa Pink","category":"Classic","description":"","price_cents":15990,"active":false,"stock":999}`, true)

	require.Equal(t, 200, rec.Code, rec.Body.String())
	assert.Equal(t, 3, s.repo.Products[0].Stock())
	assert.False(t, s.repo.Products[0].Active())
	assert.Equal(t, int64(15990), s.repo.Products[0].PriceCents())
}

func TestAdminCatalog_UpdateRequiresActiveFlagSoOmissionNeverDeactivates(t *testing.T) {
	s := newShop(t)
	id := s.create(t)

	rec := s.do("PUT", "/v1/admin/products/"+id, `{"name":"Classic Clog Rosa","category":"Classic","price_cents":15990}`, true)

	assert.Equal(t, 400, rec.Code)
	assert.True(t, s.repo.Products[0].Active())
}

func TestAdminCatalog_UpdateUnknownIs404(t *testing.T) {
	s := newShop(t)
	rec := s.do("PUT", "/v1/admin/products/nao-existe", `{"name":"X","category":"Classic","price_cents":100,"active":true}`, true)
	assert.Equal(t, 404, rec.Code)
}

func TestAdminCatalog_AdjustStockRecordsAuthenticatedAdmin(t *testing.T) {
	s := newShop(t)
	id := s.create(t)

	rec := s.do("PATCH", "/v1/admin/products/"+id+"/stock", `{"delta":-1,"reason":"venda no balcão"}`, true)

	require.Equal(t, 200, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"stock":2}`, rec.Body.String())
	require.Len(t, s.repo.Movements, 1)
	assert.Equal(t, s.adminID, s.repo.Movements[0].AdminUID, "quem fez vem da sessão, não do corpo")
}

func TestAdminCatalog_AdjustStockIgnoresAdminFromBody(t *testing.T) {
	s := newShop(t)
	id := s.create(t)

	s.do("PATCH", "/v1/admin/products/"+id+"/stock", `{"delta":1,"reason":"x","admin_uid":"outra-pessoa"}`, true)

	require.Len(t, s.repo.Movements, 1)
	assert.Equal(t, s.adminID, s.repo.Movements[0].AdminUID)
}

func TestAdminCatalog_AdjustStockErrors(t *testing.T) {
	s := newShop(t)
	id := s.create(t)
	url := "/v1/admin/products/" + id + "/stock"

	assert.Equal(t, 409, s.do("PATCH", url, `{"delta":-10,"reason":"venda"}`, true).Code, "estoque insuficiente")
	assert.Equal(t, 400, s.do("PATCH", url, `{"delta":1}`, true).Code, "sem motivo")
	assert.Equal(t, 400, s.do("PATCH", url, `{"delta":0,"reason":"x"}`, true).Code, "delta zero")
	assert.Equal(t, 404, s.do("PATCH", "/v1/admin/products/nao-existe/stock", `{"delta":1,"reason":"x"}`, true).Code)
	assert.Empty(t, s.repo.Movements)
}

func TestAdminCatalog_WritesRequireJSONContentType(t *testing.T) {
	s := newShop(t)
	rec := s.do("POST", "/v1/admin/products", newProduct, true, func(r *nethttp.Request) { r.Header.Set("Content-Type", "text/plain") })
	assert.Equal(t, 400, rec.Code)
	assert.Empty(t, s.repo.Products)
}

var _ = ports.DefaultLimit
