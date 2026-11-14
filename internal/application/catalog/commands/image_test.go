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

func withProduct(t *testing.T) (*catalogtest.Repo, *mediatest.Storage, string) {
	t.Helper()
	repo, uid := &catalogtest.Repo{}, ""
	p, err := commands.SaveHandler{Repository: repo}.Handle(context.Background(), validCmd())
	require.NoError(t, err)
	uid = p.UID()
	return repo, mediatest.New(), uid
}

func TestSetImage_StoresTheFileAndLinksItToTheProduct(t *testing.T) {
	repo, store, uid := withProduct(t)

	p, err := commands.SetImageHandler{Products: repo, Storage: store}.Handle(context.Background(), uid, realPNG(t))

	require.NoError(t, err)
	assert.True(t, media.ValidKey(p.Image()), p.Image())
	assert.True(t, strings.HasSuffix(p.Image(), ".png"), "a extensão vem do CONTEÚDO")
	assert.Contains(t, store.Files, p.Image())
	assert.Equal(t, p.Image(), repo.Products[0].Image())
}

func TestSetImage_ReplacingDeletesTheOldFile(t *testing.T) {
	repo, store, uid := withProduct(t)
	h := commands.SetImageHandler{Products: repo, Storage: store}
	first, err := h.Handle(context.Background(), uid, realPNG(t))
	require.NoError(t, err)

	second, err := h.Handle(context.Background(), uid, realPNG(t))

	require.NoError(t, err)
	assert.NotEqual(t, first.Image(), second.Image())
	assert.NotContains(t, store.Files, first.Image(), "o arquivo antigo foi apagado")
	assert.Contains(t, store.Files, second.Image())
	assert.Len(t, store.Files, 1, "sem arquivos órfãos")
}

func TestSetImage_InvalidFileChangesNothing(t *testing.T) {
	repo, store, uid := withProduct(t)

	_, err := commands.SetImageHandler{Products: repo, Storage: store}.Handle(context.Background(), uid, []byte(`<svg><script>alert(1)</script></svg>`))

	var req httperr.RequiredError
	require.True(t, errors.As(err, &req))
	assert.Empty(t, store.Files)
	assert.Empty(t, repo.Products[0].Image())
}

func TestSetImage_UnknownProductIsNotFoundAndWritesNothing(t *testing.T) {
	repo, store, _ := withProduct(t)

	_, err := commands.SetImageHandler{Products: repo, Storage: store}.Handle(context.Background(), "nao-existe", realPNG(t))

	var nf httperr.NotFoundError
	require.True(t, errors.As(err, &nf))
	assert.Empty(t, store.Files, "não grava arquivo para um produto que não existe")
}

func TestSetImage_RepositoryFailureRemovesTheFileJustSaved(t *testing.T) {
	repo, store, uid := withProduct(t)
	repo.Err = errors.New("banco fora do ar")

	_, err := commands.SetImageHandler{Products: repo, Storage: store}.Handle(context.Background(), uid, realPNG(t))

	require.Error(t, err)
	assert.Empty(t, store.Files, "sem órfão: o arquivo novo foi desfeito")
}

func TestSetImage_StorageFailureLeavesTheProductUntouched(t *testing.T) {
	repo, store, uid := withProduct(t)
	store.SaveErr = mediatest.ErrDisk

	_, err := commands.SetImageHandler{Products: repo, Storage: store}.Handle(context.Background(), uid, realPNG(t))

	require.Error(t, err)
	assert.Empty(t, repo.Products[0].Image())
}

func TestSetImage_FailingToDeleteTheOldFileDoesNotFailTheUpload(t *testing.T) {
	repo, store, uid := withProduct(t)
	h := commands.SetImageHandler{Products: repo, Storage: store}
	_, err := h.Handle(context.Background(), uid, realPNG(t))
	require.NoError(t, err)
	store.DeleteErr = mediatest.ErrDisk

	p, err := h.Handle(context.Background(), uid, realPNG(t))

	require.NoError(t, err, "a foto nova já está no ar; sobra um arquivo velho, não um erro para o usuário")
	assert.Contains(t, store.Files, p.Image())
}

func TestRemoveImage_ClearsTheLinkAndDeletesTheFile(t *testing.T) {
	repo, store, uid := withProduct(t)
	p, err := commands.SetImageHandler{Products: repo, Storage: store}.Handle(context.Background(), uid, realPNG(t))
	require.NoError(t, err)

	got, err := commands.RemoveImageHandler{Products: repo, Storage: store}.Handle(context.Background(), uid)

	require.NoError(t, err)
	assert.Empty(t, got.Image())
	assert.NotContains(t, store.Files, p.Image())
}

func TestRemoveImage_WithoutPhotoIsFineAndUnknownIsNotFound(t *testing.T) {
	repo, store, uid := withProduct(t)
	h := commands.RemoveImageHandler{Products: repo, Storage: store}

	_, err := h.Handle(context.Background(), uid)
	assert.NoError(t, err)

	_, err = h.Handle(context.Background(), "nao-existe")
	var nf httperr.NotFoundError
	assert.True(t, errors.As(err, &nf))
}
