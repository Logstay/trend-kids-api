package http_test

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Logstay/trend-kids-api/internal/adapters/http"
	"github.com/Logstay/trend-kids-api/internal/adapters/security"
	identitycmd "github.com/Logstay/trend-kids-api/internal/application/identity/commands"
	sellercmd "github.com/Logstay/trend-kids-api/internal/application/sellers/commands"
	sellerq "github.com/Logstay/trend-kids-api/internal/application/sellers/queries"
	identityports "github.com/Logstay/trend-kids-api/internal/ports/identity"
	"github.com/Logstay/trend-kids-api/internal/ports/identity/identitytest"
	"github.com/Logstay/trend-kids-api/internal/ports/sellers/sellerstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sellerDesk struct {
	handler nethttp.Handler
	cookie  *nethttp.Cookie
	repo    *sellerstest.Repo
}

func newSellerDesk(t *testing.T) sellerDesk {
	t.Helper()
	admins := &identitytest.Repo{}
	hasher := security.NewArgon2Hasher()
	tokens, err := security.NewTokenService("0123456789abcdef0123456789abcdef", time.Hour, time.Now)
	require.NoError(t, err)
	require.NoError(t, identitycmd.CreateAdminHandler{Repository: admins, Hasher: hasher}.
		Handle(context.Background(), identityports.CreateAdminCommand{Email: "dono@trendkids.com.br", Password: adminPassword}))

	repo := &sellerstest.Repo{}
	h := http.NewRouter(http.Deps{
		AllowedOrigins: []string{"http://localhost:3000"},
		Login:          identitycmd.LoginHandler{Repository: admins, Hasher: hasher, Tokens: tokens},
		Tokens:         tokens,
		LoginLimit:     100,
		Sellers: &http.SellersRoutes{
			List:   sellerq.ListHandler{Repository: repo},
			Create: sellercmd.CreateHandler{Repository: repo},
			Update: sellercmd.UpdateHandler{Repository: repo},
			Delete: sellercmd.DeleteHandler{Repository: repo},
		},
	})
	rec := post(h, "/v1/admin/login", loginBody("dono@trendkids.com.br", adminPassword))
	require.Equal(t, 204, rec.Code)
	return sellerDesk{handler: h, cookie: sessionCookie(rec), repo: repo}
}

func (d sellerDesk) do(method, url, body string, withSession bool, mods ...func(*nethttp.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if withSession {
		req.AddCookie(d.cookie)
	}
	for _, m := range mods {
		m(req)
	}
	rec := httptest.NewRecorder()
	d.handler.ServeHTTP(rec, req)
	return rec
}

func (d sellerDesk) create(t *testing.T, name, whats string) string {
	t.Helper()
	rec := d.do("POST", "/v1/admin/sellers", `{"name":"`+name+`","whatsapp":"`+whats+`"}`, true)
	require.Equal(t, 201, rec.Code, rec.Body.String())
	return d.repo.Sellers[len(d.repo.Sellers)-1].UID()
}

func TestSellers_PublicListShowsOnlyActiveWithWaMeNumberAndNothingInternal(t *testing.T) {
	d := newSellerDesk(t)
	d.create(t, "Ana", "(83) 99999-1234")
	bia := d.create(t, "Bia", "83988887766")
	require.Equal(t, 200, d.do("PUT", "/v1/admin/sellers/"+bia, `{"name":"Bia","whatsapp":"83988887766","active":false}`, true).Code)

	rec := d.do("GET", "/v1/sellers", "", false)

	require.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `"name":"Ana"`)
	assert.Contains(t, body, `"whatsapp":"5583999991234"`, "pronto para o wa.me")
	assert.Contains(t, body, `"whatsapp_display":"(83) 99999-1234"`)
	assert.NotContains(t, body, "Bia", "inativa não aparece para o público")
	assert.NotContains(t, body, `"active"`, "o público não vê campos internos")
	assert.Contains(t, rec.Header().Get("Cache-Control"), "max-age")
}

func TestSellers_PublicListEmptyIsArrayNotNull(t *testing.T) {
	d := newSellerDesk(t)
	assert.JSONEq(t, `{"items":[]}`, d.do("GET", "/v1/sellers", "", false).Body.String())
}

func TestSellers_AdminRoutesRequireSession(t *testing.T) {
	d := newSellerDesk(t)
	for _, r := range [][2]string{
		{"GET", "/v1/admin/sellers"}, {"POST", "/v1/admin/sellers"}, {"PUT", "/v1/admin/sellers/x"}, {"DELETE", "/v1/admin/sellers/x"},
	} {
		assert.Equal(t, 401, d.do(r[0], r[1], `{"name":"Ana","whatsapp":"83999991234"}`, false).Code, r[0]+" "+r[1])
	}
	assert.Empty(t, d.repo.Sellers)
}

func TestSellers_AdminListIncludesInactiveWithTheirState(t *testing.T) {
	d := newSellerDesk(t)
	d.create(t, "Ana", "83999991234")
	bia := d.create(t, "Bia", "83988887766")
	d.do("PUT", "/v1/admin/sellers/"+bia, `{"name":"Bia","whatsapp":"83988887766","active":false}`, true)

	rec := d.do("GET", "/v1/admin/sellers", "", true)

	require.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), `"name":"Bia"`)
	assert.Contains(t, rec.Body.String(), `"active":false`)
	assert.Contains(t, rec.Body.String(), `"active":true`)
}

func TestSellers_CreateErrors(t *testing.T) {
	d := newSellerDesk(t)
	d.create(t, "Ana", "83999991234")

	assert.Equal(t, 400, d.do("POST", "/v1/admin/sellers", `{"name":"","whatsapp":"83999991234"}`, true).Code, "sem nome")
	assert.Equal(t, 400, d.do("POST", "/v1/admin/sellers", `{"name":"Bia","whatsapp":"123"}`, true).Code, "whatsapp ruim")
	assert.Equal(t, 400, d.do("POST", "/v1/admin/sellers", `{"name":"Bia"}`, true).Code, "sem whatsapp")
	assert.Equal(t, 400, d.do("POST", "/v1/admin/sellers", `{`, true).Code, "json quebrado")
	assert.Equal(t, 409, d.do("POST", "/v1/admin/sellers", `{"name":"Bia","whatsapp":"+55 (83) 99999-1234"}`, true).Code, "número repetido")
	assert.Equal(t, 400, d.do("POST", "/v1/admin/sellers", `{"name":"Bia","whatsapp":"83988887766"}`, true,
		func(r *nethttp.Request) { r.Header.Set("Content-Type", "text/plain") }).Code, "exige JSON")
	assert.Len(t, d.repo.Sellers, 1)
}

func TestSellers_PutRequiresActiveFlagSoOmissionNeverDeactivates(t *testing.T) {
	d := newSellerDesk(t)
	uid := d.create(t, "Ana", "83999991234")

	rec := d.do("PUT", "/v1/admin/sellers/"+uid, `{"name":"Ana Paula","whatsapp":"83999991234"}`, true)

	assert.Equal(t, 400, rec.Code)
	assert.True(t, d.repo.Sellers[0].Active())
	assert.Equal(t, "Ana", d.repo.Sellers[0].Name())
}

func TestSellers_PutUpdatesAndUnknownIs404(t *testing.T) {
	d := newSellerDesk(t)
	uid := d.create(t, "Ana", "83999991234")

	rec := d.do("PUT", "/v1/admin/sellers/"+uid, `{"name":"Ana Paula","whatsapp":"(83) 98888-7766","active":true}`, true)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"name":"Ana Paula"`)
	assert.Equal(t, "83988887766", d.repo.Sellers[0].WhatsApp())

	assert.Equal(t, 404, d.do("PUT", "/v1/admin/sellers/nao-existe", `{"name":"X Y","whatsapp":"83999991234","active":true}`, true).Code)
}

func TestSellers_DeleteReturns204ThenNotFound(t *testing.T) {
	d := newSellerDesk(t)
	uid := d.create(t, "Ana", "83999991234")

	assert.Equal(t, 204, d.do("DELETE", "/v1/admin/sellers/"+uid, "", true).Code)
	assert.Equal(t, 404, d.do("DELETE", "/v1/admin/sellers/"+uid, "", true).Code)
	assert.Empty(t, d.repo.Sellers)
}
