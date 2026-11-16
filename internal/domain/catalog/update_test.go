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

func TestProductImages_NewHasNoneAndEditingKeepsThemInOrder(t *testing.T) {
	assert.Empty(t, existing(t).Images(), "produto novo nasce sem fotos")
	assert.Empty(t, existing(t).Image(), "e sem capa")

	withPhotos := catalog.Restore(catalog.Snapshot{
		UID: "u1", Name: "Classic Clog Rosa", Slug: "classic-clog-rosa", Category: "Classic", PriceCents: 18990,
		Images: []string{"a.webp", "b.webp", "c.webp"},
	})
	assert.Equal(t, []string{"a.webp", "b.webp", "c.webp"}, withPhotos.Images())
	assert.Equal(t, "a.webp", withPhotos.Image(), "a primeira é a capa")

	f := catalog.NewProductFactoryFrom(withPhotos)
	require.NoError(t, f.PriceCents(15990))
	assert.Equal(t, withPhotos.Images(), f.Build().Images(), "editar o preço não perde as fotos")
}

func TestProductImages_ReturnedSliceIsACopy(t *testing.T) {
	p := catalog.Restore(catalog.Snapshot{UID: "u1", Name: "X", Slug: "x", Category: "Classic", PriceCents: 1, Images: []string{"a.webp", "b.webp"}})

	got := p.Images()
	got[0] = "adulterada.webp"

	assert.Equal(t, "a.webp", p.Images()[0], "mexer no que foi devolvido não altera o produto")
}
