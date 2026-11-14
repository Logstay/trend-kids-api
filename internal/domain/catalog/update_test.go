package catalog_test

import (
	"testing"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func existing(t *testing.T) catalog.Product {
	t.Helper()
	f := catalog.NewProductFactory()
	require.NoError(t, f.Name("Classic Clog Rosa"))
	require.NoError(t, f.Category("Classic"))
	require.NoError(t, f.PriceCents(18990))
	require.NoError(t, f.Stock(5))
	return f.Build()
}

func TestFactoryFrom_KeepsIdentityAndStockWhenEditing(t *testing.T) {
	old := existing(t)

	f := catalog.NewProductFactoryFrom(old)
	require.NoError(t, f.PriceCents(15990))
	updated := f.Build()

	assert.Equal(t, old.UID(), updated.UID())
	assert.Equal(t, old.Slug(), updated.Slug())
	assert.Equal(t, 5, updated.Stock())
	assert.Equal(t, int64(15990), updated.PriceCents())
	assert.Equal(t, int64(18990), old.PriceCents(), "o original não é alterado")
}

func TestFactoryFrom_RenamingKeepsSlugStable(t *testing.T) {
	f := catalog.NewProductFactoryFrom(existing(t))

	require.NoError(t, f.Name("Classic Clog Rosa Pink"))

	assert.Equal(t, "Classic Clog Rosa Pink", f.Build().Name())
	assert.Equal(t, "classic-clog-rosa", f.Build().Slug(), "link já compartilhado não pode quebrar")
}

func TestFactory_ActiveCanBeToggled(t *testing.T) {
	f := catalog.NewProductFactoryFrom(existing(t))
	f.Active(false)
	assert.False(t, f.Build().Active())
}

func TestFactoryFrom_StillValidates(t *testing.T) {
	f := catalog.NewProductFactoryFrom(existing(t))
	assert.Error(t, f.PriceCents(0))
	assert.Error(t, f.Name(" "))
}
