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
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
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
			List:          catalogq.ListHandler{Repository: repo},
			Create:        catalogcmd.SaveHandler{Repository: repo},
			Update:        catalogcmd.UpdateHandler{Repository: repo},
			AdjustStock:   catalogcmd.AdjustStockHandler{Repository: repo},
			AddImage:      catalogcmd.AddImageHandler{Products: repo, Storage: store},
			RemoveImage:   catalogcmd.RemoveImageHandler{Products: repo, Storage: store},
			ReorderImages: catalogcmd.ReorderImagesHandler{Products: repo},
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

	req := httptest.NewRequest("POST", "/v1/admin/products/"+uid+"/images", &body)
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

type galleryJSON struct {
	Image  string `json:"image"`
	Images []struct {
		Key string `json:"key"`
		URL string `json:"url"`
	} `json:"images"`
}

func parse(t *testing.T, rec *httptest.ResponseRecorder) galleryJSON {
	t.Helper()
	var g galleryJSON
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &g), rec.Body.String())
	return g
}

func (g gallery) addPhoto(t *testing.T) galleryJSON {
	t.Helper()
	rec := g.upload(g.uid, "image", "foto.png", pngData(t), true)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	return parse(t, rec)
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

func TestImages_UploadsAccumulateInOrderAndTheFirstIsTheCover(t *testing.T) {
	g := newGallery(t)

	one := g.addPhoto(t)
	two := g.addPhoto(t)

	require.Len(t, one.Images, 1)
	require.Len(t, two.Images, 2)
	assert.Regexp(t, `^/uploads/[0-9a-f-]{36}\.png$`, two.Images[0].URL, "nome gerado pelo servidor")
	assert.Equal(t, one.Images[0].Key, two.Images[0].Key, "a primeira continua sendo a primeira")
	assert.Equal(t, two.Images[0].URL, two.Image, "image = capa")
	assert.Len(t, g.files(t), 2)

	pub := httptest.NewRecorder()
	g.handler.ServeHTTP(pub, httptest.NewRequest("GET", "/v1/products", nil))
	assert.Contains(t, pub.Body.String(), `"images":[{"key":"`+two.Images[0].Key, "a vitrine pública mostra a galeria")
}

func TestImages_ProductWithoutPhotosHasEmptyCoverAndEmptyGallery(t *testing.T) {
	g := newGallery(t)
	pub := httptest.NewRecorder()
	g.handler.ServeHTTP(pub, httptest.NewRequest("GET", "/v1/products", nil))
	assert.Contains(t, pub.Body.String(), `"image":""`)
	assert.Contains(t, pub.Body.String(), `"images":[]`, "lista vazia, nunca null")
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
		assert.Equal(t, 400, g.upload(g.uid, "image", "foto.png", data, true).Code, name)
	}
	assert.Empty(t, g.files(t), "nada foi gravado no disco")
	assert.Empty(t, g.repo.Products[0].Images())
}

func TestImages_UploadRejectsHugeBodies(t *testing.T) {
	g := newGallery(t)
	huge := append(pngData(t), bytes.Repeat([]byte{0}, 8<<20)...)

	assert.Equal(t, 400, g.upload(g.uid, "image", "grande.png", huge, true).Code)
	assert.Empty(t, g.files(t))
}

func TestImages_UploadNeedsTheImageFieldAndMultipart(t *testing.T) {
	g := newGallery(t)

	assert.Equal(t, 400, g.upload(g.uid, "outro_campo", "foto.png", pngData(t), true).Code, "campo errado")
	assert.Equal(t, 400, g.json("POST", "/v1/admin/products/"+g.uid+"/images", `{"image":"x"}`, true).Code, "não é multipart")
	assert.Empty(t, g.files(t))
}

func TestImages_UploadForUnknownProductIs404AndLeavesNoFile(t *testing.T) {
	g := newGallery(t)
	assert.Equal(t, 404, g.upload("nao-existe", "image", "foto.png", pngData(t), true).Code)
	assert.Empty(t, g.files(t))
}

func TestImages_NinthPhotoIsRefusedWithoutLeavingAFileBehind(t *testing.T) {
	g := newGallery(t)
	for i := 0; i < ports.MaxImages; i++ {
		g.addPhoto(t)
	}

	rec := g.upload(g.uid, "image", "extra.png", pngData(t), true)

	assert.Equal(t, 409, rec.Code)
	assert.Contains(t, rec.Body.String(), "limite")
	assert.Len(t, g.files(t), ports.MaxImages, "nenhum arquivo a mais no disco")
}

func TestImages_DeleteRemovesOnlyThatPhotoAndItsFile(t *testing.T) {
	g := newGallery(t)
	g.addPhoto(t)
	both := g.addPhoto(t)
	first, second := both.Images[0].Key, both.Images[1].Key

	assert.Equal(t, 401, g.json("DELETE", "/v1/admin/products/"+g.uid+"/images/"+first, "", false).Code)
	rec := g.json("DELETE", "/v1/admin/products/"+g.uid+"/images/"+first, "", true)

	require.Equal(t, 200, rec.Code, rec.Body.String())
	left := parse(t, rec)
	require.Len(t, left.Images, 1)
	assert.Equal(t, second, left.Images[0].Key)
	assert.Equal(t, left.Images[0].URL, left.Image, "a segunda virou a capa")
	assert.Equal(t, []string{second}, g.files(t))
}

func TestImages_DeleteWithUnknownOrMaliciousKeyIs404AndTouchesNothing(t *testing.T) {
	g := newGallery(t)
	keep := g.addPhoto(t).Images[0].Key
	secret := filepath.Join(filepath.Dir(g.dir), "segredo.png")
	require.NoError(t, os.WriteFile(secret, []byte("nao-apague"), 0o600))

	for _, k := range []string{"3f2b8c1e-5d4a-4b6f-9a1c-0e7d2f8b6a31.png", "..%2Fsegredo.png", "nao-e-chave.png", "index.html"} {
		assert.Equal(t, 404, g.json("DELETE", "/v1/admin/products/"+g.uid+"/images/"+k, "", true).Code, k)
	}
	assert.Equal(t, 404, g.json("DELETE", "/v1/admin/products/nao-existe/images/"+keep, "", true).Code)
	assert.Equal(t, []string{keep}, g.files(t))
	still, err := os.ReadFile(secret)
	require.NoError(t, err)
	assert.Equal(t, "nao-apague", string(still))
}

func TestImages_ReorderMakesAnotherPhotoTheCover(t *testing.T) {
	g := newGallery(t)
	g.addPhoto(t)
	g.addPhoto(t)
	three := g.addPhoto(t)
	a, b, c := three.Images[0].Key, three.Images[1].Key, three.Images[2].Key
	body := `{"keys":["` + c + `","` + a + `","` + b + `"]}`

	assert.Equal(t, 401, g.json("PUT", "/v1/admin/products/"+g.uid+"/images/order", body, false).Code)
	rec := g.json("PUT", "/v1/admin/products/"+g.uid+"/images/order", body, true)

	require.Equal(t, 200, rec.Code, rec.Body.String())
	got := parse(t, rec)
	require.Len(t, got.Images, 3)
	assert.Equal(t, []string{c, a, b}, []string{got.Images[0].Key, got.Images[1].Key, got.Images[2].Key})
	assert.Equal(t, got.Images[0].URL, got.Image, "a nova primeira é a capa")
}

func TestImages_ReorderRejectsBadRequests(t *testing.T) {
	g := newGallery(t)
	g.addPhoto(t)
	two := g.addPhoto(t)
	a, b := two.Images[0].Key, two.Images[1].Key
	url := "/v1/admin/products/" + g.uid + "/images/order"

	assert.Equal(t, 400, g.json("PUT", url, `{"keys":["`+a+`"]}`, true).Code, "faltando uma")
	assert.Equal(t, 400, g.json("PUT", url, `{"keys":["`+a+`","`+a+`"]}`, true).Code, "repetida")
	assert.Equal(t, 400, g.json("PUT", url, `{"keys":["`+a+`","../x.png"]}`, true).Code, "chave com ../")
	assert.Equal(t, 400, g.json("PUT", url, `{}`, true).Code, "sem keys")
	assert.Equal(t, 400, g.json("PUT", url, `{`, true).Code, "json quebrado")
	assert.Equal(t, 404, g.json("PUT", "/v1/admin/products/nao-existe/images/order", `{"keys":["`+a+`","`+b+`"]}`, true).Code)
	assert.Equal(t, a, g.repo.Products[0].Images()[0], "a ordem não mudou")
}

func TestImages_ServedWithSafeHeadersAndTheRightType(t *testing.T) {
	g := newGallery(t)
	data := pngData(t)
	url := parse(t, g.upload(g.uid, "image", "a.png", data, true)).Images[0].URL

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
