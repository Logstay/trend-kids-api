//go:build integration

package identity_test

import (
	"context"
	"errors"
	"os"
	"testing"

	pgidentity "github.com/Logstay/trend-kids-api/internal/adapters/postgres/identity"
	"github.com/Logstay/trend-kids-api/internal/domain/identity"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRepo(t *testing.T) *pgidentity.Repository {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL não definida")
	}
	db, err := sqlx.Connect("postgres", url)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	migration, err := os.ReadFile("../../../../config/migrations/V2__create_admin_users.sql")
	require.NoError(t, err)
	_, err = db.Exec(`DROP TABLE IF EXISTS admin_users`)
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	return pgidentity.NewRepository(db)
}

func newAdmin(t *testing.T, email string) identity.Admin {
	t.Helper()
	f := identity.NewAdminFactory()
	require.NoError(t, f.Email(email))
	require.NoError(t, f.PasswordHash("$argon2id$fake"))
	return f.Build()
}

func TestRepository_SaveAndFindByEmail(t *testing.T) {
	repo := newRepo(t)
	saved := newAdmin(t, "dono@trendkids.com.br")
	require.NoError(t, repo.Save(context.Background(), saved))

	got, err := repo.FindByEmail(context.Background(), "dono@trendkids.com.br")

	require.NoError(t, err)
	assert.Equal(t, saved.UID(), got.UID())
	assert.Equal(t, "$argon2id$fake", got.PasswordHash())
}

func TestRepository_FindByEmail_NotFound(t *testing.T) {
	_, err := newRepo(t).FindByEmail(context.Background(), "ninguem@x.com")

	var nf httperr.NotFoundError
	assert.True(t, errors.As(err, &nf))
}

func TestRepository_ExistsByEmail(t *testing.T) {
	repo := newRepo(t)
	require.NoError(t, repo.Save(context.Background(), newAdmin(t, "dono@trendkids.com.br")))

	yes, err := repo.ExistsByEmail(context.Background(), "dono@trendkids.com.br")
	require.NoError(t, err)
	no, err := repo.ExistsByEmail(context.Background(), "outro@x.com")
	require.NoError(t, err)

	assert.True(t, yes)
	assert.False(t, no)
}
