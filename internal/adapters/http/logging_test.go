package http_test

import (
	"bytes"
	"context"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/adapters/http"
	"github.com/Logstay/trend-kids-api/internal/application/catalog/queries"
	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	"github.com/Logstay/trend-kids-api/internal/ports/catalog/catalogtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type panickingList struct{}

func (panickingList) Handle(context.Context, ports.Filter) ([]catalog.Product, error) {
	panic("falha inesperada")
}

func loggedRouter(t *testing.T, list http.ListUseCase) (nethttp.Handler, *safeBuffer) {
	t.Helper()
	out := &safeBuffer{}
	repo := &catalogtest.Repo{}
	if list == nil {
		list = queries.ListHandler{Repository: repo}
	}
	return http.NewRouter(http.Deps{
		List:           list,
		Get:            queries.GetHandler{Repository: repo},
		AllowedOrigins: []string{"http://localhost:3000"},
		Logger:         slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}), out
}

func get(h nethttp.Handler, url string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", url, nil)
	req.RemoteAddr = "10.1.2.3:5555"
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLogging_OneLinePerRequestWithWhatYouNeedToDebug(t *testing.T) {
	h, out := loggedRouter(t, nil)

	get(h, "/v1/products?category=Classic&limit=5", "User-Agent", "node")

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	require.Len(t, lines, 1, out.String())
	line := lines[0]
	assert.Contains(t, line, "level=INFO")
	assert.Contains(t, line, "msg=request")
	assert.Contains(t, line, "method=GET")
	assert.Contains(t, line, "path=/v1/products")
	assert.Contains(t, line, `query="category=Classic&limit=5"`)
	assert.Contains(t, line, "status=200")
	assert.Contains(t, line, "ip=10.1.2.3")
	assert.Contains(t, line, "ua=node", "dá para distinguir o servidor do Next (node) de um navegador")
	assert.Contains(t, line, "latency_ms=")
}

func TestLogging_LevelFollowsStatus(t *testing.T) {
	h, out := loggedRouter(t, nil)

	get(h, "/v1/products")            // 200
	get(h, "/v1/products/nao-existe") // 404
	get(h, "/rota-que-nao-existe")    // 404 do roteador

	log := out.String()
	assert.Contains(t, log, "level=INFO msg=request method=GET path=/v1/products status=200")
	assert.Contains(t, log, "level=WARN msg=request method=GET path=/v1/products/nao-existe status=404")
	assert.Contains(t, log, "level=WARN msg=request method=GET path=/rota-que-nao-existe status=404")
}

func TestLogging_PanicBecomes500LoggedAsError(t *testing.T) {
	h, out := loggedRouter(t, panickingList{})

	rec := get(h, "/v1/products")

	assert.Equal(t, 500, rec.Code)
	assert.Contains(t, out.String(), "level=ERROR msg=request")
	assert.Contains(t, out.String(), "status=500")
}

// O 403 por CORS é a falha mais silenciosa do sistema (POST do site recusado sem corpo): o log precisa mostrá-la.
func TestLogging_CORSRejectionIsVisibleWithTheOrigin(t *testing.T) {
	h, out := loggedRouter(t, nil)

	rec := get(h, "/v1/products", "Origin", "https://outro-dominio.com")

	assert.Equal(t, 403, rec.Code)
	assert.Contains(t, out.String(), "status=403")
	assert.Contains(t, out.String(), "origin=https://outro-dominio.com")
}

func TestLogging_NeverWritesSecretsOrBodies(t *testing.T) {
	h, out := loggedRouter(t, nil)
	req := httptest.NewRequest("POST", "/v1/admin/login", strings.NewReader(`{"email":"dono@x.com","password":"senha-super-secreta"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "session=token-do-cookie-secreto")
	req.Header.Set("Authorization", "Bearer outro-segredo")
	h.ServeHTTP(httptest.NewRecorder(), req)

	log := out.String()
	assert.NotEmpty(t, log)
	for _, secret := range []string{"senha-super-secreta", "dono@x.com", "token-do-cookie-secreto", "outro-segredo"} {
		assert.NotContains(t, log, secret)
	}
}

func TestLogging_LongQueryIsTruncated(t *testing.T) {
	h, out := loggedRouter(t, nil)

	get(h, "/v1/products?q="+strings.Repeat("a", 1000))

	assert.Less(t, len(out.String()), 700, "uma query gigante não vira uma linha gigante")
	assert.Contains(t, out.String(), "…")
}

func TestLogging_NilLoggerMeansSilentAndSafe(t *testing.T) {
	repo := &catalogtest.Repo{}
	h := http.NewRouter(http.Deps{
		List: queries.ListHandler{Repository: repo}, Get: queries.GetHandler{Repository: repo},
		AllowedOrigins: []string{"http://localhost:3000"},
	})
	assert.NotPanics(t, func() { get(h, "/v1/products") })
}
