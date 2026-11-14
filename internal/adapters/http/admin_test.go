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
	"github.com/Logstay/trend-kids-api/internal/application/identity/commands"
	ports "github.com/Logstay/trend-kids-api/internal/ports/identity"
	"github.com/Logstay/trend-kids-api/internal/ports/identity/identitytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const adminPassword = "uma-senha-bem-longa"

type adminEnv struct {
	handler nethttp.Handler
	tokens  *security.TokenService
	adminID string
}

func newAdminServer(t *testing.T, loginLimit int) adminEnv {
	t.Helper()
	return newAdminServerBehind(t, loginLimit, nil)
}

func newAdminServerBehind(t *testing.T, loginLimit int, trustedProxies []string) adminEnv {
	t.Helper()
	repo := &identitytest.Repo{}
	hasher := security.NewArgon2Hasher()
	tokens, err := security.NewTokenService("0123456789abcdef0123456789abcdef", time.Hour, time.Now)
	require.NoError(t, err)
	require.NoError(t, commands.CreateAdminHandler{Repository: repo, Hasher: hasher}.
		Handle(context.Background(), ports.CreateAdminCommand{Email: "dono@trendkids.com.br", Password: adminPassword}))

	h := http.NewRouter(http.Deps{
		AllowedOrigins: []string{"http://localhost:3000"},
		Login:          commands.LoginHandler{Repository: repo, Hasher: hasher, Tokens: tokens},
		Tokens:         tokens,
		CookieSecure:   true,
		LoginLimit:     loginLimit,
		LoginWindow:    time.Minute,
		TrustedProxies: trustedProxies,
	})
	return adminEnv{handler: h, tokens: tokens, adminID: repo.Admins[0].UID()}
}

func post(h nethttp.Handler, url, body string, mods ...func(*nethttp.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.1:1234"
	for _, m := range mods {
		m(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func loginBody(email, password string) string {
	b, _ := json.Marshal(map[string]string{"email": email, "password": password})
	return string(b)
}

func sessionCookie(rec *httptest.ResponseRecorder) *nethttp.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	return nil
}

func TestLogin_SetsHttpOnlySecureCookieAndHidesTokenFromBody(t *testing.T) {
	e := newAdminServer(t, 10)

	rec := post(e.handler, "/v1/admin/login", loginBody("dono@trendkids.com.br", adminPassword))

	require.Equal(t, 204, rec.Code)
	c := sessionCookie(rec)
	require.NotNil(t, c)
	assert.True(t, c.HttpOnly)
	assert.True(t, c.Secure)
	assert.Equal(t, nethttp.SameSiteLaxMode, c.SameSite)
	assert.Equal(t, "/", c.Path)
	uid, err := e.tokens.Parse(c.Value)
	require.NoError(t, err)
	assert.Equal(t, e.adminID, uid)
	assert.Empty(t, rec.Body.String())
}

func TestLogin_WrongPasswordIs401WithoutCookie(t *testing.T) {
	e := newAdminServer(t, 10)

	rec := post(e.handler, "/v1/admin/login", loginBody("dono@trendkids.com.br", "senha-errada-123"))

	assert.Equal(t, 401, rec.Code)
	assert.Nil(t, sessionCookie(rec))
	assert.JSONEq(t, `{"code":401,"error":"e-mail ou senha inválidos"}`, rec.Body.String())
}

func TestLogin_MalformedBodyIs400(t *testing.T) {
	e := newAdminServer(t, 10)
	for name, body := range map[string]string{"json quebrado": "{", "sem senha": `{"email":"a@b.com"}`, "vazio": ""} {
		assert.Equal(t, 400, post(e.handler, "/v1/admin/login", body).Code, name)
	}
}

func TestLogin_RequiresJSONContentType(t *testing.T) {
	e := newAdminServer(t, 10)

	rec := post(e.handler, "/v1/admin/login", loginBody("dono@trendkids.com.br", adminPassword),
		func(r *nethttp.Request) { r.Header.Set("Content-Type", "text/plain") })

	assert.Equal(t, 400, rec.Code)
}

func TestLogin_RateLimitedPerClientIP(t *testing.T) {
	e := newAdminServer(t, 3)
	bad := loginBody("dono@trendkids.com.br", "senha-errada-123")

	for i := 0; i < 3; i++ {
		require.Equal(t, 401, post(e.handler, "/v1/admin/login", bad).Code)
	}
	blocked := post(e.handler, "/v1/admin/login", bad)
	otherIP := post(e.handler, "/v1/admin/login", bad, func(r *nethttp.Request) { r.RemoteAddr = "10.0.0.2:99" })

	assert.Equal(t, 429, blocked.Code)
	assert.Equal(t, 401, otherIP.Code, "outro IP tem a própria cota")
}

func TestLogin_RateLimitIgnoresSpoofedForwardedFor(t *testing.T) {
	e := newAdminServer(t, 2)
	bad := loginBody("dono@trendkids.com.br", "senha-errada-123")
	spoof := func(ip string) func(*nethttp.Request) {
		return func(r *nethttp.Request) { r.Header.Set("X-Forwarded-For", ip) }
	}

	post(e.handler, "/v1/admin/login", bad, spoof("1.1.1.1"))
	post(e.handler, "/v1/admin/login", bad, spoof("2.2.2.2"))

	assert.Equal(t, 429, post(e.handler, "/v1/admin/login", bad, spoof("3.3.3.3")).Code)
}

func getWith(h nethttp.Handler, url string, cookie *nethttp.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", url, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMe_RequiresValidSession(t *testing.T) {
	e := newAdminServer(t, 10)
	login := post(e.handler, "/v1/admin/login", loginBody("dono@trendkids.com.br", adminPassword))
	good := sessionCookie(login)
	tampered := &nethttp.Cookie{Name: "session", Value: good.Value[:len(good.Value)-2] + "xx"}

	assert.Equal(t, 401, getWith(e.handler, "/v1/admin/me", nil).Code, "sem cookie")
	assert.Equal(t, 401, getWith(e.handler, "/v1/admin/me", tampered).Code, "cookie adulterado")
	ok := getWith(e.handler, "/v1/admin/me", good)
	require.Equal(t, 200, ok.Code)
	assert.JSONEq(t, `{"id":"`+e.adminID+`"}`, ok.Body.String())
}

func TestLogout_ClearsCookie(t *testing.T) {
	e := newAdminServer(t, 10)

	rec := post(e.handler, "/v1/admin/logout", "")

	assert.Equal(t, 204, rec.Code)
	c := sessionCookie(rec)
	require.NotNil(t, c)
	assert.Equal(t, -1, c.MaxAge)
	assert.True(t, c.HttpOnly)
}

func TestCORS_AllowsCredentialsForConfiguredOrigin(t *testing.T) {
	e := newAdminServer(t, 10)
	req := httptest.NewRequest("OPTIONS", "/v1/admin/login", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	rec := httptest.NewRecorder()

	e.handler.ServeHTTP(rec, req)

	assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
	assert.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
}

// Atrás do Next (proxy): sem isso todo login pareceria vir de um IP só e um atacante travaria o dono.
func TestLogin_RateLimitUsesForwardedForOnlyFromTrustedProxy(t *testing.T) {
	e := newAdminServerBehind(t, 2, []string{"10.0.0.1"}) // post() usa RemoteAddr 10.0.0.1
	bad := loginBody("dono@trendkids.com.br", "senha-errada-123")
	as := func(ip string) func(*nethttp.Request) {
		return func(r *nethttp.Request) { r.Header.Set("X-Forwarded-For", ip) }
	}

	post(e.handler, "/v1/admin/login", bad, as("5.5.5.5"))
	post(e.handler, "/v1/admin/login", bad, as("5.5.5.5"))

	assert.Equal(t, 429, post(e.handler, "/v1/admin/login", bad, as("5.5.5.5")).Code, "o atacante é limitado")
	assert.Equal(t, 401, post(e.handler, "/v1/admin/login", bad, as("6.6.6.6")).Code, "o dono, de outro IP, não é afetado")
}

func TestLogin_ForwardedForFromUntrustedPeerStillIgnored(t *testing.T) {
	e := newAdminServerBehind(t, 2, []string{"10.0.0.1"})
	bad := loginBody("dono@trendkids.com.br", "senha-errada-123")
	direct := func(ip string) func(*nethttp.Request) {
		return func(r *nethttp.Request) { r.RemoteAddr = "9.9.9.9:1"; r.Header.Set("X-Forwarded-For", ip) }
	}

	post(e.handler, "/v1/admin/login", bad, direct("1.1.1.1"))
	post(e.handler, "/v1/admin/login", bad, direct("2.2.2.2"))

	assert.Equal(t, 429, post(e.handler, "/v1/admin/login", bad, direct("3.3.3.3")).Code)
}
