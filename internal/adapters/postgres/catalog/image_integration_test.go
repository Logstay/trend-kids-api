//go:build integration

package catalog_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func key(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d.webp", n) }

func isKind[T error](err error) bool {
	var target T
	return errors.As(err, &target)
}

func TestAddImage_AppendsInOrderAndTheFirstIsTheCover(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)

	for i := 1; i <= 3; i++ {
		require.NoError(t, repo.AddImage(ctx, p.UID(), key(i)))
	}

	got, err := repo.GetByUID(ctx, p.UID())
	require.NoError(t, err)
	assert.Equal(t, []string{key(1), key(2), key(3)}, got.Images())
	assert.Equal(t, key(1), got.Image(), "capa = primeira")
}

func TestAddImage_StopsAtTheLimit(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	for i := 1; i <= ports.MaxImages; i++ {
		require.NoError(t, repo.AddImage(ctx, p.UID(), key(i)))
	}

	err := repo.AddImage(ctx, p.UID(), key(99))

	assert.True(t, isKind[httperr.ConflictError](err), "%v", err)
	var n int
	require.NoError(t, db.Get(&n, `SELECT count(*) FROM product_images`))
	assert.Equal(t, ports.MaxImages, n)
}

// Dez pessoas adicionando fotos ao mesmo tempo quando só cabe mais uma: só uma pode vencer.
func TestAddImage_ConcurrentAddsNeverExceedTheLimit(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	for i := 1; i < ports.MaxImages; i++ {
		require.NoError(t, repo.AddImage(ctx, p.UID(), key(i)))
	}

	const tries = 10
	var wg sync.WaitGroup
	errs := make(chan error, tries)
	for i := 0; i < tries; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- repo.AddImage(ctx, p.UID(), key(100+i))
		}(i)
	}
	wg.Wait()
	close(errs)

	ok, conflicts := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			ok++
		case isKind[httperr.ConflictError](err):
			conflicts++
		default:
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	assert.Equal(t, 1, ok)
	assert.Equal(t, tries-1, conflicts)
	var n int
	require.NoError(t, db.Get(&n, `SELECT count(*) FROM product_images`))
	assert.Equal(t, ports.MaxImages, n)
}

func TestAddImage_SameKeyTwiceIsConflictAndUnknownProductIsNotFound(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	require.NoError(t, repo.AddImage(ctx, p.UID(), key(1)))

	assert.True(t, isKind[httperr.ConflictError](repo.AddImage(ctx, p.UID(), key(1))))
	assert.True(t, isKind[httperr.NotFoundError](repo.AddImage(ctx, "00000000-0000-0000-0000-000000000000", key(2))))
	assert.True(t, isKind[httperr.NotFoundError](repo.AddImage(ctx, "isto-nao-e-uuid", key(2))))
}

func TestRemoveImage_RemovesAndKeepsTheOrderContiguous(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	for i := 1; i <= 3; i++ {
		require.NoError(t, repo.AddImage(ctx, p.UID(), key(i)))
	}

	require.NoError(t, repo.RemoveImage(ctx, p.UID(), key(2)))

	got, _ := repo.GetByUID(ctx, p.UID())
	assert.Equal(t, []string{key(1), key(3)}, got.Images())
	var positions []int
	require.NoError(t, db.Select(&positions, `SELECT position FROM product_images ORDER BY position`))
	assert.Equal(t, []int{0, 1}, positions, "sem buracos: a próxima foto entra na posição certa")
	require.NoError(t, repo.AddImage(ctx, p.UID(), key(4)))
	got, _ = repo.GetByUID(ctx, p.UID())
	assert.Equal(t, []string{key(1), key(3), key(4)}, got.Images())
}

func TestRemoveImage_UnknownKeyOrAnotherProductsKeyIsNotFound(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()
	a := seed(t, repo, "Produto A", "Classic", 1000)
	b := seed(t, repo, "Produto B", "Classic", 2000)
	require.NoError(t, repo.AddImage(ctx, a.UID(), key(1)))

	assert.True(t, isKind[httperr.NotFoundError](repo.RemoveImage(ctx, a.UID(), key(9))))
	assert.True(t, isKind[httperr.NotFoundError](repo.RemoveImage(ctx, b.UID(), key(1))), "a foto é do produto A")
	got, _ := repo.GetByUID(ctx, a.UID())
	assert.Equal(t, []string{key(1)}, got.Images(), "nada foi removido")
}

func TestReorderImages_AppliesTheNewOrder(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	for i := 1; i <= 3; i++ {
		require.NoError(t, repo.AddImage(ctx, p.UID(), key(i)))
	}

	require.NoError(t, repo.ReorderImages(ctx, p.UID(), []string{key(3), key(1), key(2)}))

	got, _ := repo.GetByUID(ctx, p.UID())
	assert.Equal(t, []string{key(3), key(1), key(2)}, got.Images())
	assert.Equal(t, key(3), got.Image(), "a nova primeira é a capa")
}

func TestReorderImages_OnlyAcceptsAPermutationOfTheCurrentPhotos(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	for i := 1; i <= 3; i++ {
		require.NoError(t, repo.AddImage(ctx, p.UID(), key(i)))
	}

	for name, order := range map[string][]string{
		"faltando uma":     {key(1), key(2)},
		"com uma de fora":  {key(1), key(2), key(3), key(4)},
		"repetida":         {key(1), key(1), key(2)},
		"uma desconhecida": {key(1), key(2), key(9)},
		"vazia":            {},
	} {
		err := repo.ReorderImages(ctx, p.UID(), order)
		assert.True(t, isKind[httperr.RequiredError](err), name)
	}
	got, _ := repo.GetByUID(ctx, p.UID())
	assert.Equal(t, []string{key(1), key(2), key(3)}, got.Images(), "a ordem não mudou")
	assert.True(t, isKind[httperr.NotFoundError](repo.ReorderImages(ctx, "00000000-0000-0000-0000-000000000000", []string{key(1)})))
}

func TestImages_AppearInEveryReadPathForManyProducts(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()
	a := seed(t, repo, "Produto A", "Classic", 1000)
	b := seed(t, repo, "Produto B", "Charms", 2000)
	c := seed(t, repo, "Produto C", "Classic", 3000) // sem fotos
	require.NoError(t, repo.AddImage(ctx, a.UID(), key(1)))
	require.NoError(t, repo.AddImage(ctx, a.UID(), key(2)))
	require.NoError(t, repo.AddImage(ctx, b.UID(), key(3)))

	list, err := repo.List(ctx, ports.Filter{Limit: 10, Sort: ports.SortPriceAsc})
	require.NoError(t, err)
	require.Len(t, list, 3)
	bySlug := map[string]catalog.Product{}
	for _, p := range list {
		bySlug[p.Slug()] = p
	}
	assert.Equal(t, []string{key(1), key(2)}, bySlug[a.Slug()].Images())
	assert.Equal(t, []string{key(3)}, bySlug[b.Slug()].Images())
	assert.Empty(t, bySlug[c.Slug()].Images())

	one, err := repo.GetBySlug(ctx, a.Slug())
	require.NoError(t, err)
	assert.Equal(t, []string{key(1), key(2)}, one.Images())
}

func TestUpdate_NeverTouchesThePhotos(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	require.NoError(t, repo.AddImage(ctx, p.UID(), key(1)))
	require.NoError(t, repo.AddImage(ctx, p.UID(), key(2)))
	current, _ := repo.GetByUID(ctx, p.UID())

	f := catalog.NewProductFactoryFrom(current)
	require.NoError(t, f.PriceCents(15990))
	require.NoError(t, repo.Update(ctx, f.Build()))

	got, _ := repo.GetByUID(ctx, p.UID())
	assert.Equal(t, []string{key(1), key(2)}, got.Images(), "editar preço/nome não mexe nas fotos")
}

// Estresse: várias rodadas de disputa pela última vaga. Uma única rodada pode passar por sorte; vinte não.
func TestAddImage_StressManyRoundsOfRacesForTheLastSlot(t *testing.T) {
	repo, db := newRepo(t)
	ctx := context.Background()

	for round := 0; round < 20; round++ {
		p := seed(t, repo, fmt.Sprintf("Produto da rodada %02d", round), "Classic", 1000)
		for i := 1; i < ports.MaxImages; i++ {
			require.NoError(t, repo.AddImage(ctx, p.UID(), key(round*100+i)))
		}

		var wg sync.WaitGroup
		results := make(chan error, 8)
		start := make(chan struct{})
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				results <- repo.AddImage(ctx, p.UID(), key(round*100+50+i))
			}(i)
		}
		close(start)
		wg.Wait()
		close(results)

		ok := 0
		for err := range results {
			switch {
			case err == nil:
				ok++
			case isKind[httperr.ConflictError](err):
			default:
				t.Fatalf("rodada %d: erro inesperado: %v", round, err)
			}
		}
		var n int
		require.NoError(t, db.Get(&n, `SELECT count(*) FROM product_images WHERE product_uid = $1`, p.UID()))
		require.Equal(t, ports.MaxImages, n, "rodada %d passou do limite", round)
		require.Equal(t, 1, ok, "rodada %d", round)
	}
}
