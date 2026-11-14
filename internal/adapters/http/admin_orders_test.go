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
	ordercmd "github.com/Logstay/trend-kids-api/internal/application/orders/commands"
	orderq "github.com/Logstay/trend-kids-api/internal/application/orders/queries"
	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	identityports "github.com/Logstay/trend-kids-api/internal/ports/identity"
	"github.com/Logstay/trend-kids-api/internal/ports/identity/identitytest"
	"github.com/Logstay/trend-kids-api/internal/ports/orders/orderstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type desk struct {
	handler nethttp.Handler
	cookie  *nethttp.Cookie
	orders  *orderstest.Repo
	order   orders.Order
	adminID string
}

func newDesk(t *testing.T) desk {
	t.Helper()
	admins := &identitytest.Repo{}
	hasher := security.NewArgon2Hasher()
	tokens, err := security.NewTokenService("0123456789abcdef0123456789abcdef", time.Hour, time.Now)
	require.NoError(t, err)
	require.NoError(t, identitycmd.CreateAdminHandler{Repository: admins, Hasher: hasher}.
		Handle(context.Background(), identityports.CreateAdminCommand{Email: "dono@trendkids.com.br", Password: adminPassword}))

	f := orders.NewOrderFactory()
	require.NoError(t, f.Customer("Maria Silva", "(83) 99999-1234", "Manaíra"))
	require.NoError(t, f.AddItem("p1", "Classic Clog Rosa", 18990, 2))
	o, err := f.Build(orders.ShippingPolicy{FlatCents: 1500})
	require.NoError(t, err)
	repo := &orderstest.Repo{Orders: []orders.Order{o}}

	h := http.NewRouter(http.Deps{
		AllowedOrigins: []string{"http://localhost:3000"},
		Login:          identitycmd.LoginHandler{Repository: admins, Hasher: hasher, Tokens: tokens},
		Tokens:         tokens,
		LoginLimit:     100,
		AdminOrders: &http.AdminOrders{
			List:         orderq.AdminListHandler{Orders: repo},
			Get:          orderq.AdminGetHandler{Orders: repo},
			ChangeStatus: ordercmd.ChangeStatusHandler{Orders: repo},
		},
	})
	rec := post(h, "/v1/admin/login", loginBody("dono@trendkids.com.br", adminPassword))
	require.Equal(t, 204, rec.Code)
	return desk{handler: h, cookie: sessionCookie(rec), orders: repo, order: o, adminID: admins.Admins[0].UID()}
}

func (d desk) do(method, url, body string, withSession bool, mods ...func(*nethttp.Request)) *httptest.ResponseRecorder {
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

func TestAdminOrders_EveryRouteRequiresSession(t *testing.T) {
	d := newDesk(t)
	for _, r := range [][2]string{
		{"GET", "/v1/admin/orders"},
		{"GET", "/v1/admin/orders/" + d.order.Code()},
		{"PATCH", "/v1/admin/orders/" + d.order.Code() + "/status"},
	} {
		assert.Equal(t, 401, d.do(r[0], r[1], `{"status":"paid"}`, false).Code, r[0]+" "+r[1])
	}
	assert.Empty(t, d.orders.Changes, "nada mudou sem sessão")
}

func TestAdminOrders_ListShowsCustomerPhoneAndPassesFilters(t *testing.T) {
	d := newDesk(t)

	rec := d.do("GET", "/v1/admin/orders?status=new&limit=5&offset=2", "", true)

	require.Equal(t, 200, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"`+d.order.Code()+`"`)
	assert.Contains(t, rec.Body.String(), `"customer_phone":"83999991234"`, "o painel precisa do telefone para contatar")
	assert.Contains(t, rec.Body.String(), `"customer_name":"Maria Silva"`)
	assert.Equal(t, "new", d.orders.LastFilter.Status)
	assert.Equal(t, 5, d.orders.LastFilter.Limit)
	assert.Equal(t, 2, d.orders.LastFilter.Offset)
}

func TestAdminOrders_ListInvalidParamsAreBadRequest(t *testing.T) {
	d := newDesk(t)
	for _, url := range []string{"/v1/admin/orders?status=pago", "/v1/admin/orders?limit=abc", "/v1/admin/orders?offset=-1"} {
		assert.Equal(t, 400, d.do("GET", url, "", true).Code, url)
	}
}

func TestAdminOrders_EmptyListIsArrayNotNull(t *testing.T) {
	d := newDesk(t)
	assert.JSONEq(t, `{"items":[]}`, d.do("GET", "/v1/admin/orders?status=delivered", "", true).Body.String())
}

func TestAdminOrders_GetReturnsOrderWithHistory(t *testing.T) {
	d := newDesk(t)
	d.do("PATCH", "/v1/admin/orders/"+d.order.Code()+"/status", `{"status":"paid"}`, true)

	rec := d.do("GET", "/v1/admin/orders/"+d.order.Code(), "", true)

	require.Equal(t, 200, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"status":"paid"`)
	assert.Contains(t, rec.Body.String(), `"events":[{"from":"new","to":"paid","admin_id":"`+d.adminID+`"`)
}

func TestAdminOrders_GetUnknownIs404(t *testing.T) {
	d := newDesk(t)
	assert.Equal(t, 404, d.do("GET", "/v1/admin/orders/TK22222222", "", true).Code)
}

func TestAdminOrders_ChangeStatusRecordsAuthenticatedAdminNotTheBody(t *testing.T) {
	d := newDesk(t)

	rec := d.do("PATCH", "/v1/admin/orders/"+d.order.Code()+"/status", `{"status":"paid","admin_uid":"outra-pessoa"}`, true)

	require.Equal(t, 200, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"status":"paid"`)
	require.Len(t, d.orders.Changes, 1)
	assert.Equal(t, d.adminID, d.orders.Changes[0].AdminUID)
}

func TestAdminOrders_ChangeStatusErrors(t *testing.T) {
	d := newDesk(t)
	url := "/v1/admin/orders/" + d.order.Code() + "/status"

	assert.Equal(t, 409, d.do("PATCH", url, `{"status":"delivered"}`, true).Code, "pula etapas")
	assert.Equal(t, 400, d.do("PATCH", url, `{"status":"pago"}`, true).Code, "status desconhecido")
	assert.Equal(t, 400, d.do("PATCH", url, `{}`, true).Code, "sem status")
	assert.Equal(t, 400, d.do("PATCH", url, `{`, true).Code, "json quebrado")
	assert.Equal(t, 404, d.do("PATCH", "/v1/admin/orders/TK22222222/status", `{"status":"paid"}`, true).Code)
	assert.Equal(t, 400, d.do("PATCH", url, `{"status":"paid"}`, true, func(r *nethttp.Request) { r.Header.Set("Content-Type", "text/plain") }).Code)
	assert.Equal(t, orders.StatusNew, d.orders.Orders[0].Status(), "nada mudou nos casos de erro")
}
