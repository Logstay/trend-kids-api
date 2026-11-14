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
	settingscmd "github.com/Logstay/trend-kids-api/internal/application/settings/commands"
	settingsq "github.com/Logstay/trend-kids-api/internal/application/settings/queries"
	identityports "github.com/Logstay/trend-kids-api/internal/ports/identity"
	"github.com/Logstay/trend-kids-api/internal/ports/identity/identitytest"
	"github.com/Logstay/trend-kids-api/internal/ports/settings/settingstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type config struct {
	handler nethttp.Handler
	cookie  *nethttp.Cookie
	repo    *settingstest.Repo
	adminID string
}

func newConfig(t *testing.T) config {
	t.Helper()
	admins := &identitytest.Repo{}
	hasher := security.NewArgon2Hasher()
	tokens, err := security.NewTokenService("0123456789abcdef0123456789abcdef", time.Hour, time.Now)
	require.NoError(t, err)
	require.NoError(t, identitycmd.CreateAdminHandler{Repository: admins, Hasher: hasher}.
		Handle(context.Background(), identityports.CreateAdminCommand{Email: "dono@trendkids.com.br", Password: adminPassword}))

	repo := settingstest.New()
	h := http.NewRouter(http.Deps{
		AllowedOrigins: []string{"http://localhost:3000"},
		Login:          identitycmd.LoginHandler{Repository: admins, Hasher: hasher, Tokens: tokens},
		Tokens:         tokens,
		LoginLimit:     100,
		Settings: &http.SettingsRoutes{
			Get:    settingsq.GetHandler{Repository: repo},
			Update: settingscmd.UpdateHandler{Repository: repo},
		},
	})
	rec := post(h, "/v1/admin/login", loginBody("dono@trendkids.com.br", adminPassword))
	require.Equal(t, 204, rec.Code)
	return config{handler: h, cookie: sessionCookie(rec), repo: repo, adminID: admins.Admins[0].UID()}
}

func (c config) do(method, url, body string, withSession bool, mods ...func(*nethttp.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if withSession {
		req.AddCookie(c.cookie)
	}
	for _, m := range mods {
		m(req)
	}
	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	return rec
}

const fullSettings = `{"name":"Trend Kids JP","city":"João Pessoa · PB","description":"Crocs infantis","whatsapp":"(83) 99999-1234","phone":"(83) 3222-1234","instagram":"@trendkidsjp"}`

func TestSettings_PublicGetNeedsNoSessionAndStartsWithoutContacts(t *testing.T) {
	c := newConfig(t)

	rec := c.do("GET", "/v1/settings", "", false)

	require.Equal(t, 200, rec.Code)
	assert.Contains(t, rec.Body.String(), `"name":"Trend Kids JP"`)
	assert.Contains(t, rec.Body.String(), `"whatsapp":""`, "ainda não configurado")
	assert.Contains(t, rec.Header().Get("Cache-Control"), "max-age")
}

func TestSettings_PutRequiresSession(t *testing.T) {
	c := newConfig(t)

	rec := c.do("PUT", "/v1/admin/settings", fullSettings, false)

	assert.Equal(t, 401, rec.Code)
	assert.Equal(t, 0, c.repo.Updates, "nada foi gravado sem sessão")
}

func TestSettings_PutThenPublicGetShowsFormattedContacts(t *testing.T) {
	c := newConfig(t)

	put := c.do("PUT", "/v1/admin/settings", fullSettings, true)
	require.Equal(t, 200, put.Code, put.Body.String())
	rec := c.do("GET", "/v1/settings", "", false)

	body := rec.Body.String()
	assert.Contains(t, body, `"whatsapp":"5583999991234"`, "pronto para o wa.me")
	assert.Contains(t, body, `"whatsapp_display":"(83) 99999-1234"`)
	assert.Contains(t, body, `"phone":"(83) 3222-1234"`)
	assert.Contains(t, body, `"phone_tel":"+558332221234"`)
	assert.Contains(t, body, `"instagram":"trendkidsjp"`)
	assert.Contains(t, body, `"instagram_url":"https://www.instagram.com/trendkidsjp"`)
}

func TestSettings_PutRecordsAdminFromSessionNotBody(t *testing.T) {
	c := newConfig(t)

	body := strings.Replace(fullSettings, `"name"`, `"admin_uid":"outra-pessoa","name"`, 1)
	rec := c.do("PUT", "/v1/admin/settings", body, true)

	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{c.adminID}, c.repo.UpdatedBy)
}

func TestSettings_PutMustSendEveryFieldSoOmissionNeverWipesAContact(t *testing.T) {
	c := newConfig(t)
	require.Equal(t, 200, c.do("PUT", "/v1/admin/settings", fullSettings, true).Code)

	// sem a chave "whatsapp": se isso fosse aceito, o contato seria apagado em silêncio
	rec := c.do("PUT", "/v1/admin/settings", `{"name":"Trend Kids JP","city":"JP","description":"","phone":"","instagram":""}`, true)

	assert.Equal(t, 400, rec.Code)
	assert.Equal(t, "83999991234", c.repo.Current.WhatsApp(), "o WhatsApp continua lá")
}

func TestSettings_EmptyStringClearsAContactOnPurpose(t *testing.T) {
	c := newConfig(t)
	require.Equal(t, 200, c.do("PUT", "/v1/admin/settings", fullSettings, true).Code)

	rec := c.do("PUT", "/v1/admin/settings", `{"name":"Trend Kids JP","city":"JP","description":"","whatsapp":"","phone":"","instagram":""}`, true)

	require.Equal(t, 200, rec.Code)
	assert.Empty(t, c.repo.Current.WhatsApp())
	assert.Contains(t, rec.Body.String(), `"whatsapp":""`)
}

func TestSettings_PutValidationErrorsAre400WithAClearMessage(t *testing.T) {
	c := newConfig(t)
	bad := func(field, value string) string {
		return strings.Replace(fullSettings, field, value, 1)
	}
	tests := map[string]string{
		"whatsapp inválido":  bad(`(83) 99999-1234`, `123`),
		"instagram inválido": bad(`@trendkidsjp`, `https://outro.com/x`),
		"nome vazio":         bad(`"Trend Kids JP"`, `""`),
		"json quebrado":      `{`,
	}
	for name, body := range tests {
		rec := c.do("PUT", "/v1/admin/settings", body, true)
		assert.Equal(t, 400, rec.Code, name)
	}
	assert.Equal(t, 0, c.repo.Updates)

	rec := c.do("PUT", "/v1/admin/settings", bad(`(83) 99999-1234`, `123`), true)
	assert.Contains(t, rec.Body.String(), "whatsapp inválido")
}

func TestSettings_PutRequiresJSONContentType(t *testing.T) {
	c := newConfig(t)
	rec := c.do("PUT", "/v1/admin/settings", fullSettings, true, func(r *nethttp.Request) { r.Header.Set("Content-Type", "text/plain") })
	assert.Equal(t, 400, rec.Code)
}
