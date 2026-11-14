//go:build integration

package catalog_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	keyA = "3f2b8c1e-5d4a-4b6f-9a1c-0e7d2f8b6a31.jpg"
	keyB = "7a1d9e44-1c2b-4f3a-8d5e-6b0c9a2f4e18.webp"
)

func TestSetImage_ReturnsThePreviousKeyAndPersistsTheNewOne(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)

	prev, err := repo.SetImage(ctx, p.UID(), keyA)
	require.NoError(t, err)
	assert.Empty(t, prev, "não havia foto")

	prev, err = repo.SetImage(ctx, p.UID(), keyB)
	require.NoError(t, err)
	assert.Equal(t, keyA, prev, "devolve a anterior para o arquivo velho ser apagado")

	got, _ := repo.GetByUID(ctx, p.UID())
	assert.Equal(t, keyB, got.Image())
}

func TestSetImage_EmptyKeyRemovesThePhoto(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	_, _ = repo.SetImage(ctx, p.UID(), keyA)

	prev, err := repo.SetImage(ctx, p.UID(), "")

	require.NoError(t, err)
	assert.Equal(t, keyA, prev)
	got, _ := repo.GetByUID(ctx, p.UID())
	assert.Empty(t, got.Image())
}

func TestImage_AppearsInEveryReadPath(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	_, _ = repo.SetImage(ctx, p.UID(), keyA)

	bySlug, err := repo.GetBySlug(ctx, p.Slug())
	require.NoError(t, err)
	list, err := repo.List(ctx, ports.Filter{Limit: 10})
	require.NoError(t, err)

	assert.Equal(t, keyA, bySlug.Image())
	require.Len(t, list, 1)
	assert.Equal(t, keyA, list[0].Image())
}

func TestUpdate_NeverTouchesThePhoto(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	_, _ = repo.SetImage(ctx, p.UID(), keyA)
	current, _ := repo.GetByUID(ctx, p.UID())

	f := catalog.NewProductFactoryFrom(current)
	require.NoError(t, f.PriceCents(15990))
	require.NoError(t, repo.Update(ctx, f.Build()))

	got, _ := repo.GetByUID(ctx, p.UID())
	assert.Equal(t, keyA, got.Image(), "editar preço/nome não apaga a foto")
	assert.Equal(t, int64(15990), got.PriceCents())
}

func TestSetImage_UnknownOrMalformedUIDIsNotFound(t *testing.T) {
	repo, _ := newRepo(t)
	var nf httperr.NotFoundError

	_, err := repo.SetImage(context.Background(), "00000000-0000-0000-0000-000000000000", keyA)
	assert.True(t, errors.As(err, &nf))
	_, err = repo.SetImage(context.Background(), "isto-nao-e-uuid", keyA)
	assert.True(t, errors.As(err, &nf))
}
