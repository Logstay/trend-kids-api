//go:build integration

package catalog_test

import (
	"context"
	"errors"
	"os"
	"testing"

	pgcatalog "github.com/Logstay/trend-kids-api/internal/adapters/postgres/catalog"
	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRepo(t *testing.T) (*pgcatalog.Repository, *sqlx.DB) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL não definida")
	}
	db, err := sqlx.Connect("postgres", url)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`DROP TABLE IF EXISTS product_images, order_events, order_items, orders, stock_movements`)
	require.NoError(t, err)
	_, err = db.Exec(`DROP TABLE IF EXISTS products`)
	require.NoError(t, err)
	for _, file := range []string{"V1__create_products.sql", "V3__create_stock_movements.sql", "V8__add_product_image.sql", "V9__create_product_images.sql"} {
		migration, err := os.ReadFile("../../../../config/migrations/" + file)
		require.NoError(t, err)
		_, err = db.Exec(string(migration))
		require.NoError(t, err)
	}

	return pgcatalog.NewRepository(db), db
}

func seed(t *testing.T, repo *pgcatalog.Repository, name, category string, cents int64) catalog.Product {
	t.Helper()
	f := catalog.NewProductFactory()
	require.NoError(t, f.Name(name))
	require.NoError(t, f.Category(category))
	require.NoError(t, f.PriceCents(cents))
	require.NoError(t, f.Stock(4))
	p := f.Build()
	require.NoError(t, repo.Save(context.Background(), p))
	return p
}

func TestRepository_SaveAndGetBySlug_RoundTrip(t *testing.T) {
	repo, _ := newRepo(t)
	saved := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)

	got, err := repo.GetBySlug(context.Background(), "classic-clog-rosa")

	require.NoError(t, err)
	assert.Equal(t, saved.UID(), got.UID())
	assert.Equal(t, int64(18990), got.PriceCents())
	assert.Equal(t, 4, got.Stock())
	assert.True(t, got.Active())
	assert.False(t, got.CreatedAt().IsZero())
}

func TestRepository_GetBySlug_NotFound(t *testing.T) {
	repo, _ := newRepo(t)

	_, err := repo.GetBySlug(context.Background(), "nao-existe")

	var nf httperr.NotFoundError
	assert.True(t, errors.As(err, &nf))
}

func TestRepository_ExistsBySlug(t *testing.T) {
	repo, _ := newRepo(t)
	seed(t, repo, "Kit 5 Charms", "Charms", 4990)

	yes, err := repo.ExistsBySlug(context.Background(), "kit-5-charms")
	require.NoError(t, err)
	no, err := repo.ExistsBySlug(context.Background(), "outro")
	require.NoError(t, err)

	assert.True(t, yes)
	assert.False(t, no)
}

func TestRepository_List_FiltersSortsAndPaginates(t *testing.T) {
	repo, db := newRepo(t)
	seed(t, repo, "Classic Clog Rosa", "Classic", 20000)
	seed(t, repo, "Classic Clog Azul", "Classic", 15000)
	seed(t, repo, "Sandália Crocband", "Sandálias", 12000)
	seed(t, repo, "Kit 5 Charms", "Charms", 4990)
	_, err := db.Exec(`UPDATE products SET active = false WHERE slug = 'kit-5-charms'`)
	require.NoError(t, err)
	ctx := context.Background()

	byCategory, err := repo.List(ctx, ports.Filter{Category: "Classic", Sort: ports.SortPriceAsc, Limit: 10})
	require.NoError(t, err)
	require.Len(t, byCategory, 2)
	assert.Equal(t, "classic-clog-azul", byCategory[0].Slug(), "ordenado por menor preço")

	byQuery, err := repo.List(ctx, ports.Filter{Query: "sandalia", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, byQuery, 1, "busca ignora acento e caixa")

	all, err := repo.List(ctx, ports.Filter{Sort: ports.SortPriceDesc, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, all, 3, "produto inativo não aparece")
	assert.Equal(t, "classic-clog-rosa", all[0].Slug())

	page, err := repo.List(ctx, ports.Filter{Sort: ports.SortPriceDesc, Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, "classic-clog-azul", page[0].Slug())
}
