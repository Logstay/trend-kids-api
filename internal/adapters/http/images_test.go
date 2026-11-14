package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	nethttp "net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Logstay/trend-kids-api/internal/adapters/http"
	"github.com/Logstay/trend-kids-api/internal/adapters/security"
	"github.com/Logstay/trend-kids-api/internal/adapters/storage/local"
	catalogcmd "github.com/Logstay/trend-kids-api/internal/application/catalog/commands"
	catalogq "github.com/Logstay/trend-kids-api/internal/application/catalog/queries"
	identitycmd "github.com/Logstay/trend-kids-api/internal/application/identity/commands"
	"github.com/Logstay/trend-kids-api/internal/ports/catalog/catalogtest"
	identityports "github.com/Logstay/trend-kids-api/internal/ports/identity"
	"github.com/Logstay/trend-kids-api/internal/ports/identity/identitytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type gallery struct {
	handler nethttp.Handler
	cookie  *nethttp.Cookie
	repo    *catalogtest.Repo
	dir     string
	uid     string
}

func newGallery(t *testing.T) gallery {
	t.Helper()
	admins := &identitytest.Repo{}
	hasher := security.NewArgon2Hasher()
	tokens, err := security.NewTokenService("0123456789abcdef0123456789abcdef", time.Hour, time.Now)
	require.NoError(t, err)
	require.NoError(t, identitycmd.CreateAdminHandler{Repository: admins, Hasher: hasher}.
		Handle(context.Background(), identityports.CreateAdminCommand{Email: "dono@trendkids.com.br", Password: adminPassword}))

	dir := filepath.Join(t.TempDir(), "uploads")
	store, err := local.New(dir)
	require.NoError(t, err)
	repo := &catalogtest.Repo{}
	h := http.NewRouter(http.Deps{
		AllowedOrigins: []string{"http://localhost:3000"},
		List:           catalogq.ListHandler{Repository: repo},
		Get:            catalogq.GetHandler{Repository: repo},
		Login:          identitycmd.LoginHandler{Repository: admins, Hasher: hasher, Tokens: tokens},
		Tokens:         tokens,
		LoginLimit:     100,
		Images:         store,
		UploadsDir:     store.Dir(),
		Catalog: &http.AdminCatalog{
			List:        catalogq.ListHandler{Repository: repo},
			Create:      catalogcmd.SaveHandler{Repository: repo},
			Update:      catalogcmd.UpdateHandler{Repository: repo},
			AdjustStock: catalogcmd.AdjustStockHandler{Repository: repo},
			SetImage:    catalogcmd.SetImageHandler{Products: repo, Storage: store},
			RemoveImage: catalogcmd.RemoveImageHandler{Products: repo, Storage: store},
		},
	})
	rec := post(h, "/v1/admin/login", loginBody("dono@trendkids.com.br", adminPassword))
	require.Equal(t, 204, rec.Code)
	g := gallery{handler: h, cookie: sessionCookie(rec), repo: repo, dir: dir}

	created := g.json("POST", "/v1/admin/products", `{"name":"Classic Clog Rosa","category":"Classic","price_cents":18990,"stock":3}`, true)
	require.Equal(t, 201, created.Code, created.Body.String())
	g.uid = repo.Products[0].UID()
	return g
}

func (g gallery) json(method, url, body string, session bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if session {
		req.AddCookie(g.cookie)
	}
	rec := httptest.NewRecorder()
	g.handler.ServeHTTP(rec, req)
	return rec
}

func (g gallery) upload(uid, field, filename string, data []byte, session bool) *httptest.ResponseRecorder {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	// o navegador pode dizer qualquer Content-Type: o servidor tem que ignorar e olhar o conteúdo
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{`form-data; name="` + field + `"; filename="` + filename + `"`}
	h["Content-Type"] = []string{"image/png"}
	part, _ := w.CreatePart(h)
	_, _ = part.Write(data)
	_ = w.Close()

	req := httptest.NewRequest("POST", "/v1/admin/products/"+uid+"/image", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if session {
		req.AddCookie(g.cookie)
	}
	rec := httptest.NewRecorder()
	g.handler.ServeHTTP(rec, req)
	return rec
}

func (g gallery) files(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(g.dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func pngData(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8))))
	return buf.Bytes()
}

func imageURL(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var p struct {
		Image string `json:"image"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	return p.Image
}

func TestImages_UploadRequiresSessionAndWritesNothingWithout(t *testing.T) {
	g := newGallery(t)
	rec := g.upload(g.uid, "image", "foto.png", pngData(t), false)
	assert.Equal(t, 401, rec.Code)
	assert.Empty(t, g.files(t))
}

func TestImages_UploadStoresTheFileAndTheProductShowsItsUrl(t *testing.T) {
	g := newGallery(t)

	rec := g.upload(g.uid, "image", "minha foto.png", pngData(t), true)

	require.Equal(t, 200, rec.Code, rec.Body.String())
	url := imageURL(t, rec)
	assert.Regexp(t, `^/uploads/[0-9a-f-]{36}\.png$`, url, "nome gerado pelo servidor, não o do arquivo enviado")
	assert.Len(t, g.files(t), 1)
	assert.NotContains(t, rec.Body.String(), "minha foto", "o nome enviado nunca é usado")

	pub := httptest.NewRecorder()
	g.handler.ServeHTTP(pub, httptest.NewRequest("GET", "/v1/products", nil))
	assert.Contains(t, pub.Body.String(), `"image":"`+url+`"`, "a vitrine pública já mostra a foto")
}

func TestImages_ProductWithoutPhotoHasEmptyImage(t *testing.T) {
	g := newGallery(t)
	pub := httptest.NewRecorder()
	g.handler.ServeHTTP(pub, httptest.NewRequest("GET", "/v1/products", nil))
	assert.Contains(t, pub.Body.String(), `"image":""`)
}

func TestImages_UploadRejectsFilesThatAreNotRealImages(t *testing.T) {
	g := newGallery(t)
	attacks := map[string][]byte{
		"svg com script, enviado como .png": []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"html":                              []byte(`<html><body onload="alert(1)">`),
		"executável":                        []byte("MZ\x90\x00\x03\x00\x00\x00"),
		"vazio":                             {},
	}
	for name, data := range attacks {
		rec := g.upload(g.uid, "image", "foto.png", data, true)
		assert.Equal(t, 400, rec.Code, name)
	}
	assert.Empty(t, g.files(t), "nada foi gravado no disco")
	assert.Empty(t, g.repo.Products[0].Image())
}

func TestImages_UploadRejectsHugeBodies(t *testing.T) {
	g := newGallery(t)
	huge := append(pngData(t), bytes.Repeat([]byte{0}, 8<<20)...)

	rec := g.upload(g.uid, "image", "grande.png", huge, true)

	assert.Equal(t, 400, rec.Code)
	assert.Empty(t, g.files(t))
}

func TestImages_UploadNeedsTheImageFieldAndMultipart(t *testing.T) {
	g := newGallery(t)

	assert.Equal(t, 400, g.upload(g.uid, "outro_campo", "foto.png", pngData(t), true).Code, "campo errado")
	assert.Equal(t, 400, g.json("POST", "/v1/admin/products/"+g.uid+"/image", `{"image":"x"}`, true).Code, "não é multipart")
	assert.Empty(t, g.files(t))
}

func TestImages_UploadForUnknownProductIs404AndLeavesNoFile(t *testing.T) {
	g := newGallery(t)
	rec := g.upload("nao-existe", "image", "foto.png", pngData(t), true)
	assert.Equal(t, 404, rec.Code)
	assert.Empty(t, g.files(t))
}

func TestImages_ReplacingKeepsOnlyTheNewFile(t *testing.T) {
	g := newGallery(t)
	first := imageURL(t, g.upload(g.uid, "image", "a.png", pngData(t), true))
	second := imageURL(t, g.upload(g.uid, "image", "b.png", pngData(t), true))

	assert.NotEqual(t, first, second)
	assert.Len(t, g.files(t), 1, "o arquivo antigo foi apagado")
}

func TestImages_DeleteRemovesThePhotoAndTheFile(t *testing.T) {
	g := newGallery(t)
	g.upload(g.uid, "image", "a.png", pngData(t), true)
	require.Len(t, g.files(t), 1)

	assert.Equal(t, 401, g.json("DELETE", "/v1/admin/products/"+g.uid+"/image", "", false).Code)
	rec := g.json("DELETE", "/v1/admin/products/"+g.uid+"/image", "", true)

	require.Equal(t, 200, rec.Code)
	assert.Empty(t, imageURL(t, rec))
	assert.Empty(t, g.files(t))
	assert.Equal(t, 404, g.json("DELETE", "/v1/admin/products/nao-existe/image", "", true).Code)
}

func TestImages_ServedWithSafeHeadersAndTheRightType(t *testing.T) {
	g := newGallery(t)
	data := pngData(t)
	url := imageURL(t, g.upload(g.uid, "image", "a.png", data, true))

	rec := httptest.NewRecorder()
	g.handler.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))

	require.Equal(t, 200, rec.Code)
	assert.Equal(t, data, rec.Body.Bytes())
	assert.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Contains(t, rec.Header().Get("Cache-Control"), "immutable", "o nome é único, então pode cachear para sempre")
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "sandbox")
}

func TestImages_OnlyValidKeysAreServed(t *testing.T) {
	g := newGallery(t)
	require.NoError(t, os.WriteFile(filepath.Join(g.dir, ".upload-temporario"), []byte("segredo"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(g.dir), "fora.txt"), []byte("segredo"), 0o600))

	for _, url := range []string{
		"/uploads/.upload-temporario", "/uploads/nao-e-uma-chave.png", "/uploads/..%2Ffora.txt", "/uploads/../fora.txt",
		"/uploads/%2e%2e/fora.txt", "/uploads/", "/uploads", "/uploads/index.html",
		"/uploads/3f2b8c1e-5d4a-4b6f-9a1c-0e7d2f8b6a31.png", // chave válida mas inexistente
	} {
		rec := httptest.NewRecorder()
		g.handler.ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
		assert.Equal(t, 404, rec.Code, url)
		assert.NotContains(t, rec.Body.String(), "segredo", url)
	}
}
