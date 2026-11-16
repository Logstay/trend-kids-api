package commands_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/application/catalog/commands"
	"github.com/Logstay/trend-kids-api/internal/domain/media"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	"github.com/Logstay/trend-kids-api/internal/ports/catalog/catalogtest"
	"github.com/Logstay/trend-kids-api/internal/ports/media/mediatest"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func realPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 6, 6))))
	return buf.Bytes()
}

type gallery struct {
	repo  *catalogtest.Repo
	store *mediatest.Storage
	uid   string
}

func newGallery(t *testing.T) gallery {
	t.Helper()
	repo := &catalogtest.Repo{}
	p, err := commands.SaveHandler{Repository: repo}.Handle(context.Background(), validCmd())
	require.NoError(t, err)
	return gallery{repo: repo, store: mediatest.New(), uid: p.UID()}
}

func (g gallery) add(t *testing.T) string {
	t.Helper()
	p, err := commands.AddImageHandler{Products: g.repo, Storage: g.store}.Handle(context.Background(), g.uid, realPNG(t))
	require.NoError(t, err)
	imgs := p.Images()
	return imgs[len(imgs)-1]
}

func TestAddImage_StoresTheFileAndAppendsItToTheGallery(t *testing.T) {
	g := newGallery(t)

	first := g.add(t)
	second := g.add(t)

	p := g.repo.Products[0]
	assert.Equal(t, []string{first, second}, p.Images())
	assert.Equal(t, first, p.Image(), "a primeira é a capa")
	assert.True(t, media.ValidKey(first))
	assert.True(t, strings.HasSuffix(first, ".png"), "a extensão vem do CONTEÚDO")
	assert.Len(t, g.store.Files, 2)
}

func TestAddImage_InvalidFileChangesNothing(t *testing.T) {
	g := newGallery(t)

	_, err := commands.AddImageHandler{Products: g.repo, Storage: g.store}.Handle(context.Background(), g.uid, []byte(`<svg><script>alert(1)</script></svg>`))

	var req httperr.RequiredError
	require.True(t, errors.As(err, &req))
	assert.Empty(t, g.store.Files)
	assert.Empty(t, g.repo.Products[0].Images())
}

func TestAddImage_UnknownProductIsNotFoundAndWritesNothing(t *testing.T) {
	g := newGallery(t)

	_, err := commands.AddImageHandler{Products: g.repo, Storage: g.store}.Handle(context.Background(), "nao-existe", realPNG(t))

	var nf httperr.NotFoundError
	require.True(t, errors.As(err, &nf))
	assert.Empty(t, g.store.Files, "não grava arquivo para um produto que não existe")
}

func TestAddImage_WhenTheGalleryIsFullItRefusesBeforeWritingAnyFile(t *testing.T) {
	g := newGallery(t)
	for i := 0; i < ports.MaxImages; i++ {
		g.add(t)
	}
	require.Len(t, g.store.Files, ports.MaxImages)

	_, err := commands.AddImageHandler{Products: g.repo, Storage: g.store}.Handle(context.Background(), g.uid, realPNG(t))

	var conflict httperr.ConflictError
	require.True(t, errors.As(err, &conflict))
	assert.Len(t, g.store.Files, ports.MaxImages, "nenhum arquivo a mais no disco")
	assert.Len(t, g.repo.Products[0].Images(), ports.MaxImages)
}

func TestAddImage_RepositoryFailureRemovesTheFileJustSaved(t *testing.T) {
	g := newGallery(t)
	g.repo.Err = errors.New("banco fora do ar")

	_, err := commands.AddImageHandler{Products: g.repo, Storage: g.store}.Handle(context.Background(), g.uid, realPNG(t))

	require.Error(t, err)
	assert.Empty(t, g.store.Files, "sem órfão: o arquivo novo foi desfeito")
}

func TestAddImage_StorageFailureLeavesTheGalleryUntouched(t *testing.T) {
	g := newGallery(t)
	g.store.SaveErr = mediatest.ErrDisk

	_, err := commands.AddImageHandler{Products: g.repo, Storage: g.store}.Handle(context.Background(), g.uid, realPNG(t))

	require.Error(t, err)
	assert.Empty(t, g.repo.Products[0].Images())
}

func TestRemoveImage_RemovesOnlyThatPhotoAndItsFile(t *testing.T) {
	g := newGallery(t)
	a, b, c := g.add(t), g.add(t), g.add(t)

	p, err := commands.RemoveImageHandler{Products: g.repo, Storage: g.store}.Handle(context.Background(), g.uid, b)

	require.NoError(t, err)
	assert.Equal(t, []string{a, c}, p.Images(), "as outras mantêm a ordem")
	assert.NotContains(t, g.store.Files, b)
	assert.Len(t, g.store.Files, 2)
}

func TestRemoveImage_UnknownOrMalformedKeyIsNotFoundAndTouchesNoFile(t *testing.T) {
	g := newGallery(t)
	keep := g.add(t)
	h := commands.RemoveImageHandler{Products: g.repo, Storage: g.store}
	var nf httperr.NotFoundError

	for _, bad := range []string{media.NewKey(".png"), "../../etc/passwd", "a/b.png", "", "index.html"} {
		_, err := h.Handle(context.Background(), g.uid, bad)
		assert.True(t, errors.As(err, &nf), "%q", bad)
	}
	assert.Contains(t, g.store.Files, keep)
	assert.Empty(t, g.store.Deleted, "nenhum Delete chegou ao armazenamento (chave estranha nunca vai ao disco)")
	_, err := h.Handle(context.Background(), "nao-existe", keep)
	assert.True(t, errors.As(err, &nf))
}

func TestRemoveImage_FailingToDeleteTheFileDoesNotFailTheRequest(t *testing.T) {
	g := newGallery(t)
	key := g.add(t)
	g.store.DeleteErr = mediatest.ErrDisk

	p, err := commands.RemoveImageHandler{Products: g.repo, Storage: g.store}.Handle(context.Background(), g.uid, key)

	require.NoError(t, err, "a foto já saiu da galeria; sobra um arquivo, não um erro para o usuário")
	assert.Empty(t, p.Images())
}

func TestReorderImages_AppliesTheOrderAndTheFirstBecomesTheCover(t *testing.T) {
	g := newGallery(t)
	a, b, c := g.add(t), g.add(t), g.add(t)

	p, err := commands.ReorderImagesHandler{Products: g.repo}.Handle(context.Background(), g.uid, []string{c, a, b})

	require.NoError(t, err)
	assert.Equal(t, []string{c, a, b}, p.Images())
	assert.Equal(t, c, p.Image())
}

func TestReorderImages_RejectsAnythingThatIsNotThePhotosOfTheProduct(t *testing.T) {
	g := newGallery(t)
	a, b := g.add(t), g.add(t)
	h := commands.ReorderImagesHandler{Products: g.repo}
	var req httperr.RequiredError

	for name, order := range map[string][]string{
		"faltando uma": {a}, "repetida": {a, a}, "estranha": {a, media.NewKey(".png")},
		"chave com ../": {a, "../x.png"}, "vazia": {},
	} {
		_, err := h.Handle(context.Background(), g.uid, order)
		assert.True(t, errors.As(err, &req), name)
	}
	assert.Equal(t, []string{a, b}, g.repo.Products[0].Images(), "a ordem não mudou")

	var nf httperr.NotFoundError
	_, err := h.Handle(context.Background(), "nao-existe", []string{a, b})
	assert.True(t, errors.As(err, &nf))
}
