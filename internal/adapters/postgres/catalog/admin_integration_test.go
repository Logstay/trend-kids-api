//go:build integration

package catalog_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func movements(t *testing.T, db interface {
	Select(dest any, query string, args ...any) error
}, uid string) []struct {
	Delta  int    `db:"delta"`
	Reason string `db:"reason"`
} {
	t.Helper()
	var rows []struct {
		Delta  int    `db:"delta"`
		Reason string `db:"reason"`
	}
	require.NoError(t, db.Select(&rows, `SELECT delta, reason FROM stock_movements WHERE product_uid = $1 ORDER BY id`, uid))
	return rows
}

func TestRepository_Save_RecordsInitialStockMovement(t *testing.T) {
	repo, db := newRepo(t)
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990) // estoque inicial 4

	rows := movements(t, db, p.UID())

	require.Len(t, rows, 1)
	assert.Equal(t, 4, rows[0].Delta)
	assert.Equal(t, "estoque inicial", rows[0].Reason)
}

func TestRepository_GetByUID_SeesInactiveAndRejectsBadUID(t *testing.T) {
	repo, db := newRepo(t)
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	_, err := db.Exec(`UPDATE products SET active = false WHERE uid = $1`, p.UID())
	require.NoError(t, err)

	got, err := repo.GetByUID(context.Background(), p.UID())
	require.NoError(t, err)
	assert.False(t, got.Active())

	var nf httperr.NotFoundError
	_, err = repo.GetByUID(context.Background(), uuid.NewString())
	assert.True(t, errors.As(err, &nf), "uuid inexistente")
	_, err = repo.GetByUID(context.Background(), "isto-nao-e-uuid")
	assert.True(t, errors.As(err, &nf), "uid malformado vira 404, não 500")
}

func TestRepository_Update_ChangesFieldsButNeverStockOrSlug(t *testing.T) {
	repo, _ := newRepo(t)
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	f := catalog.NewProductFactoryFrom(p)
	require.NoError(t, f.Name("Classic Clog Rosa Pink"))
	require.NoError(t, f.PriceCents(15990))
	require.NoError(t, f.Stock(999)) // tentativa de burlar o livro-razão
	f.Active(false)

	require.NoError(t, repo.Update(context.Background(), f.Build()))
	got, err := repo.GetByUID(context.Background(), p.UID())

	require.NoError(t, err)
	assert.Equal(t, "Classic Clog Rosa Pink", got.Name())
	assert.Equal(t, int64(15990), got.PriceCents())
	assert.False(t, got.Active())
	assert.Equal(t, "classic-clog-rosa", got.Slug())
	assert.Equal(t, 4, got.Stock(), "estoque só muda por movimentação")
}

func TestRepository_Update_UnknownIsNotFound(t *testing.T) {
	repo, _ := newRepo(t)
	ghost := catalog.Restore(catalog.Snapshot{UID: uuid.NewString(), Name: "x", Slug: "x", Category: "Classic", PriceCents: 1})

	err := repo.Update(context.Background(), ghost)

	var nf httperr.NotFoundError
	assert.True(t, errors.As(err, &nf))
}

func TestRepository_List_IncludeInactiveOnlyWhenAsked(t *testing.T) {
	repo, db := newRepo(t)
	seed(t, repo, "Classic Clog Rosa", "Classic", 18990)
	hidden := seed(t, repo, "Kit 5 Charms", "Charms", 4990)
	_, err := db.Exec(`UPDATE products SET active = false WHERE uid = $1`, hidden.UID())
	require.NoError(t, err)

	pub, err := repo.List(context.Background(), ports.Filter{Limit: 10})
	require.NoError(t, err)
	admin, err := repo.List(context.Background(), ports.Filter{Limit: 10, IncludeInactive: true})
	require.NoError(t, err)

	assert.Len(t, pub, 1)
	assert.Len(t, admin, 2)
}

func TestRepository_AdjustStock_UpdatesBalanceAndWritesLedger(t *testing.T) {
	repo, db := newRepo(t)
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990) // 4
	admin := uuid.NewString()

	stock, err := repo.AdjustStock(context.Background(), ports.StockMovement{ProductUID: p.UID(), Delta: -3, Reason: "venda", AdminUID: admin})

	require.NoError(t, err)
	assert.Equal(t, 1, stock)
	got, _ := repo.GetByUID(context.Background(), p.UID())
	assert.Equal(t, 1, got.Stock())
	rows := movements(t, db, p.UID())
	require.Len(t, rows, 2)
	assert.Equal(t, -3, rows[1].Delta)
	assert.Equal(t, "venda", rows[1].Reason)
	var who string
	require.NoError(t, db.Get(&who, `SELECT admin_uid::text FROM stock_movements WHERE product_uid = $1 AND delta = -3`, p.UID()))
	assert.Equal(t, admin, who)
}

func TestRepository_AdjustStock_RefusesNegativeBalanceAndLeavesNoTrace(t *testing.T) {
	repo, db := newRepo(t)
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990) // 4

	_, err := repo.AdjustStock(context.Background(), ports.StockMovement{ProductUID: p.UID(), Delta: -5, Reason: "venda"})

	var conflict httperr.ConflictError
	assert.True(t, errors.As(err, &conflict))
	got, _ := repo.GetByUID(context.Background(), p.UID())
	assert.Equal(t, 4, got.Stock())
	assert.Len(t, movements(t, db, p.UID()), 1, "só o estoque inicial")
}

func TestRepository_AdjustStock_UnknownProductIsNotFound(t *testing.T) {
	repo, _ := newRepo(t)

	_, err := repo.AdjustStock(context.Background(), ports.StockMovement{ProductUID: uuid.NewString(), Delta: 1, Reason: "x"})

	var nf httperr.NotFoundError
	assert.True(t, errors.As(err, &nf))
}

// A regra crítica: dois pedidos simultâneos para o último par não podem vender os dois.
func TestRepository_AdjustStock_ConcurrentSalesNeverOversell(t *testing.T) {
	repo, db := newRepo(t)
	p := seed(t, repo, "Classic Clog Rosa", "Classic", 18990) // estoque 4

	const buyers = 12
	var wg sync.WaitGroup
	results := make(chan error, buyers)
	for i := 0; i < buyers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.AdjustStock(context.Background(), ports.StockMovement{ProductUID: p.UID(), Delta: -1, Reason: "venda"})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	sold, refused := 0, 0
	for err := range results {
		var conflict httperr.ConflictError
		switch {
		case err == nil:
			sold++
		case errors.As(err, &conflict):
			refused++
		default:
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	got, _ := repo.GetByUID(context.Background(), p.UID())

	assert.Equal(t, 4, sold)
	assert.Equal(t, buyers-4, refused)
	assert.Equal(t, 0, got.Stock())
	assert.Len(t, movements(t, db, p.UID()), 1+4, "inicial + 4 vendas")
}
