package catalog_test

import (
	"errors"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductFactory_BuildsValidProduct(t *testing.T) {
	f := catalog.NewProductFactory()
	require.NoError(t, f.Name("Classic Clog Rosa"))
	require.NoError(t, f.Category("Classic"))
	require.NoError(t, f.PriceCents(18990))
	require.NoError(t, f.Stock(5))
	f.Description("Clog infantil")

	p := f.Build()

	assert.Equal(t, "Classic Clog Rosa", p.Name())
	assert.Equal(t, "classic-clog-rosa", p.Slug())
	assert.Equal(t, catalog.CategoryClassic, p.Category())
	assert.Equal(t, int64(18990), p.PriceCents())
	assert.Equal(t, 5, p.Stock())
	assert.NotEmpty(t, p.UID())
	assert.True(t, p.Active())
}

func TestProductFactory_SlugRemovesAccentsAndSymbols(t *testing.T) {
	f := catalog.NewProductFactory()
	require.NoError(t, f.Name("  Sandália Crocband — Infantil! "))
	assert.Equal(t, "sandalia-crocband-infantil", f.Build().Slug())
}

func TestProductFactory_RejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name string
		call func(f catalog.ProductFactory) error
	}{
		{"nome vazio", func(f catalog.ProductFactory) error { return f.Name("   ") }},
		{"categoria desconhecida", func(f catalog.ProductFactory) error { return f.Category("Chinelo") }},
		{"preço zero", func(f catalog.ProductFactory) error { return f.PriceCents(0) }},
		{"preço negativo", func(f catalog.ProductFactory) error { return f.PriceCents(-1) }},
		{"estoque negativo", func(f catalog.ProductFactory) error { return f.Stock(-1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call(catalog.NewProductFactory())
			var req httperr.RequiredError
			assert.True(t, errors.As(err, &req), "esperava RequiredError, veio %v", err)
		})
	}
}
