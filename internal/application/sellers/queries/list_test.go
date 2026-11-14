package queries_test

import (
	"context"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/application/sellers/queries"
	"github.com/Logstay/trend-kids-api/internal/domain/sellers"
	"github.com/Logstay/trend-kids-api/internal/ports/sellers/sellerstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func repoWithOneInactive(t *testing.T) *sellerstest.Repo {
	t.Helper()
	mk := func(name, whats string, active bool) sellers.Seller {
		f := sellers.NewSellerFactory()
		require.NoError(t, f.Name(name))
		require.NoError(t, f.WhatsApp(whats))
		f.Active(active)
		s, err := f.Build()
		require.NoError(t, err)
		return s
	}
	return &sellerstest.Repo{Sellers: []sellers.Seller{mk("Ana", "83999991234", true), mk("Bia", "83988887766", false)}}
}

func TestList_PublicSeesOnlyActive(t *testing.T) {
	repo := repoWithOneInactive(t)

	got, err := queries.ListHandler{Repository: repo}.Handle(context.Background(), true)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Ana", got[0].Name())
	assert.True(t, repo.LastActive)
}

func TestList_AdminSeesEveryone(t *testing.T) {
	got, err := queries.ListHandler{Repository: repoWithOneInactive(t)}.Handle(context.Background(), false)
	require.NoError(t, err)
	assert.Len(t, got, 2)
}
