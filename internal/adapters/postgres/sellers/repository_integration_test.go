//go:build integration

package sellers_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	pgsellers "github.com/Logstay/trend-kids-api/internal/adapters/postgres/sellers"
	"github.com/Logstay/trend-kids-api/internal/domain/sellers"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRepo(t *testing.T) *pgsellers.Repository {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL não definida")
	}
	db, err := sqlx.Connect("postgres", url)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`DROP TABLE IF EXISTS sellers`)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../../../config/migrations/sql/V7__create_sellers.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	return pgsellers.NewRepository(db)
}

func seller(t *testing.T, name, whatsapp string, active bool) sellers.Seller {
	t.Helper()
	f := sellers.NewSellerFactory()
	require.NoError(t, f.Name(name))
	require.NoError(t, f.WhatsApp(whatsapp))
	f.Active(active)
	s, err := f.Build()
	require.NoError(t, err)
	return s
}

func isConflict(err error) bool {
	var c httperr.ConflictError
	return errors.As(err, &c)
}

func TestSaveAndGet_RoundTrip(t *testing.T) {
	repo := newRepo(t)
	s := seller(t, "Ana Paula", "(83) 99999-1234", true)
	require.NoError(t, repo.Save(context.Background(), s))

	got, err := repo.GetByUID(context.Background(), s.UID())

	require.NoError(t, err)
	assert.Equal(t, "Ana Paula", got.Name())
	assert.Equal(t, "83999991234", got.WhatsApp())
	assert.True(t, got.Active())
	assert.False(t, got.CreatedAt().IsZero())
}

func TestList_OldestFirstAndActiveFilter(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	for _, s := range []sellers.Seller{
		seller(t, "Ana", "83999991111", true),
		seller(t, "Bia", "83999992222", false),
		seller(t, "Carla", "83999993333", true),
	} {
		require.NoError(t, repo.Save(ctx, s))
	}

	all, err := repo.List(ctx, false)
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, []string{"Ana", "Bia", "Carla"}, []string{all[0].Name(), all[1].Name(), all[2].Name()})

	active, err := repo.List(ctx, true)
	require.NoError(t, err)
	require.Len(t, active, 2)
	assert.Equal(t, "Carla", active[1].Name())
}

func TestList_EmptyIsNotNil(t *testing.T) {
	got, err := newRepo(t).List(context.Background(), true)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestSave_DuplicateWhatsAppIsConflict(t *testing.T) {
	repo := newRepo(t)
	require.NoError(t, repo.Save(context.Background(), seller(t, "Ana", "83999991234", true)))

	err := repo.Save(context.Background(), seller(t, "Bia", "83999991234", true))

	assert.True(t, isConflict(err))
}

// Duas pessoas cadastrando o mesmo número ao mesmo tempo: só uma pode vencer.
func TestSave_ConcurrentDuplicatesLetExactlyOneWin(t *testing.T) {
	repo := newRepo(t)

	const tries = 8
	var wg sync.WaitGroup
	errs := make(chan error, tries)
	for i := 0; i < tries; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- repo.Save(context.Background(), seller(t, "Ana", "83999991234", true))
		}()
	}
	wg.Wait()
	close(errs)

	ok, conflicts := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			ok++
		case isConflict(err):
			conflicts++
		default:
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	assert.Equal(t, 1, ok)
	assert.Equal(t, tries-1, conflicts)
	all, _ := repo.List(context.Background(), false)
	assert.Len(t, all, 1)
}

func TestUpdate_ChangesFieldsAndGuardsTheNumber(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	ana := seller(t, "Ana", "83999991111", true)
	require.NoError(t, repo.Save(ctx, ana))
	require.NoError(t, repo.Save(ctx, seller(t, "Bia", "83999992222", true)))

	f := sellers.NewSellerFactoryFrom(ana)
	require.NoError(t, f.Name("Ana Paula"))
	f.Active(false)
	changed, _ := f.Build()
	require.NoError(t, repo.Update(ctx, changed), "salvar com o próprio número é permitido")
	got, _ := repo.GetByUID(ctx, ana.UID())
	assert.Equal(t, "Ana Paula", got.Name())
	assert.False(t, got.Active())

	f = sellers.NewSellerFactoryFrom(ana)
	require.NoError(t, f.WhatsApp("83999992222"))
	steal, _ := f.Build()
	assert.True(t, isConflict(repo.Update(ctx, steal)), "número da Bia")
}

func TestUnknownOrMalformedUID_IsNotFound_NeverA500(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	var nf httperr.NotFoundError

	_, err := repo.GetByUID(ctx, "00000000-0000-0000-0000-000000000000")
	assert.True(t, errors.As(err, &nf))
	_, err = repo.GetByUID(ctx, "isto-nao-e-uuid")
	assert.True(t, errors.As(err, &nf))
	assert.True(t, errors.As(repo.Delete(ctx, "isto-nao-e-uuid"), &nf))
	assert.True(t, errors.As(repo.Update(ctx, seller(t, "Fantasma", "83999990000", true)), &nf))
}

func TestDelete_RemovesAndFreesTheNumber(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	s := seller(t, "Ana", "83999991234", true)
	require.NoError(t, repo.Save(ctx, s))

	require.NoError(t, repo.Delete(ctx, s.UID()))

	all, _ := repo.List(ctx, false)
	assert.Empty(t, all)
	assert.NoError(t, repo.Save(ctx, seller(t, "Outra", "83999991234", true)), "o número volta a ficar livre")
}
