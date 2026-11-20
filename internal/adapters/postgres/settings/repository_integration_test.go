//go:build integration

package settings_test

import (
	"context"
	"os"
	"testing"
	"time"

	pgsettings "github.com/Logstay/trend-kids-api/internal/adapters/postgres/settings"
	"github.com/Logstay/trend-kids-api/internal/domain/settings"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRepo(t *testing.T) (*pgsettings.Repository, *sqlx.DB) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL não definida")
	}
	db, err := sqlx.Connect("postgres", url)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`DROP TABLE IF EXISTS store_settings`)
	require.NoError(t, err)
	migration, err := os.ReadFile("../../../../config/migrations/sql/V6__create_store_settings.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	return pgsettings.NewRepository(db), db
}

func built(t *testing.T, whatsapp, instagram string) settings.Settings {
	t.Helper()
	f := settings.NewSettingsFactory()
	require.NoError(t, f.Name("Trend Kids JP"))
	require.NoError(t, f.City("João Pessoa · PB"))
	require.NoError(t, f.Description("Crocs infantis"))
	require.NoError(t, f.WhatsApp(whatsapp))
	require.NoError(t, f.Phone("(83) 3222-1234"))
	require.NoError(t, f.Instagram(instagram))
	s, err := f.Build()
	require.NoError(t, err)
	return s
}

func TestGet_ReturnsDefaultsRightAfterMigration(t *testing.T) {
	repo, _ := newRepo(t)

	got, err := repo.Get(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "Trend Kids JP", got.Name())
	assert.Equal(t, "João Pessoa · PB", got.City())
	assert.NotEmpty(t, got.Description())
	assert.Empty(t, got.WhatsApp(), "contatos começam não configurados")
	assert.Empty(t, got.Instagram())
}

func TestUpdate_RoundTripAndWhoChangedIt(t *testing.T) {
	repo, db := newRepo(t)
	admin := uuid.NewString()
	before, _ := repo.Get(context.Background())

	time.Sleep(10 * time.Millisecond)
	require.NoError(t, repo.Update(context.Background(), built(t, "(83) 99999-1234", "@trendkidsjp"), admin))
	got, err := repo.Get(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "83999991234", got.WhatsApp())
	assert.Equal(t, "8332221234", got.Phone())
	assert.Equal(t, "trendkidsjp", got.Instagram())
	assert.True(t, got.UpdatedAt().After(before.UpdatedAt()), "updated_at avançou")
	var who string
	require.NoError(t, db.Get(&who, `SELECT updated_by::text FROM store_settings`))
	assert.Equal(t, admin, who)
}

func TestUpdate_AlwaysKeepsASingleRow(t *testing.T) {
	repo, db := newRepo(t)
	for i := 0; i < 3; i++ {
		require.NoError(t, repo.Update(context.Background(), built(t, "(83) 99999-1234", "loja"), ""))
	}
	var n int
	require.NoError(t, db.Get(&n, `SELECT count(*) FROM store_settings`))
	assert.Equal(t, 1, n)

	_, err := db.Exec(`INSERT INTO store_settings (id, name, city) VALUES (2, 'x', 'y')`)
	assert.Error(t, err, "o banco recusa uma segunda linha")
}

func TestUpdate_WithoutAdminStoresNull(t *testing.T) {
	repo, db := newRepo(t)
	require.NoError(t, repo.Update(context.Background(), built(t, "", ""), ""))
	var who *string
	require.NoError(t, db.Get(&who, `SELECT updated_by::text FROM store_settings`))
	assert.Nil(t, who)
}
