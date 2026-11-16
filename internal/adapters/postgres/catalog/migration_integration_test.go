//go:build integration

package catalog_test

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func apply(t *testing.T, db *sqlx.DB, file string) {
	t.Helper()
	sqlText, err := os.ReadFile("../../../../config/migrations/" + file)
	require.NoError(t, err)
	_, err = db.Exec(string(sqlText))
	require.NoError(t, err, file)
}

// A V9 move a foto única da V8 para a galeria. Este teste usa dados REAIS: aplicar a migração só com a
// tabela vazia não exercita o INSERT ... SELECT, que é justamente a parte que pode falhar.
func TestMigrationV9_MovesTheSinglePhotoIntoTheGalleryAndDropsTheOldColumn(t *testing.T) {
	_, db := newRepo(t) // só para ter a conexão com o banco de teste
	_, err := db.Exec(`DROP TABLE IF EXISTS product_images, order_events, order_items, orders, stock_movements, products`)
	require.NoError(t, err)
	for _, f := range []string{"V1__create_products.sql", "V3__create_stock_movements.sql", "V8__add_product_image.sql"} {
		apply(t, db, f)
	}
	_, err = db.Exec(`
		INSERT INTO products (uid, name, slug, category, price_cents, stock, image_key) VALUES
		('11111111-1111-4111-8111-111111111111', 'A', 'a', 'Classic', 100, 1, 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa.webp'),
		('22222222-2222-4222-8222-222222222222', 'B', 'b', 'Classic', 100, 1, ''),
		('33333333-3333-4333-8333-333333333333', 'C', 'c', 'Charms', 100, 1, 'cccccccc-cccc-4ccc-8ccc-cccccccccccc.jpg')`)
	require.NoError(t, err)

	apply(t, db, "V9__create_product_images.sql")

	var rows []struct {
		Slug     string `db:"slug"`
		Key      string `db:"image_key"`
		Position int    `db:"position"`
	}
	require.NoError(t, db.Select(&rows, `SELECT p.slug, i.image_key, i.position FROM product_images i JOIN products p ON p.uid = i.product_uid ORDER BY p.slug`))
	require.Len(t, rows, 2, "só os produtos que tinham foto")
	assert.Equal(t, "a", rows[0].Slug)
	assert.Equal(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa.webp", rows[0].Key)
	assert.Equal(t, 0, rows[0].Position, "vira a capa")
	assert.Equal(t, "c", rows[1].Slug)

	var cols int
	require.NoError(t, db.Get(&cols, `SELECT count(*) FROM information_schema.columns WHERE table_name = 'products' AND column_name = 'image_key'`))
	assert.Equal(t, 0, cols, "a coluna antiga foi removida")

	// `make migrate` roda TODAS as migrações de novo a cada execução: precisa ser idempotente e não duplicar fotos.
	apply(t, db, "V8__add_product_image.sql")
	apply(t, db, "V9__create_product_images.sql")
	var n int
	require.NoError(t, db.Get(&n, `SELECT count(*) FROM product_images`))
	assert.Equal(t, 2, n)
}
