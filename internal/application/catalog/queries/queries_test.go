package queries_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Logstay/trend-kids-api/internal/application/catalog/queries"
	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	"github.com/Logstay/trend-kids-api/internal/ports/catalog/catalogtest"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func product(t *testing.T, name string) catalog.Product {
	t.Helper()
	f := catalog.NewProductFactory()
	require.NoError(t, f.Name(name))
	require.NoError(t, f.Category("Classic"))
	require.NoError(t, f.PriceCents(1000))
	return f.Build()
}

func TestGet_ReturnsProductBySlug(t *testing.T) {
	repo := &catalogtest.Repo{Products: []catalog.Product{product(t, "Classic Clog Rosa")}}

	p, err := queries.GetHandler{Repository: repo}.Handle(context.Background(), "classic-clog-rosa")

	require.NoError(t, err)
	assert.Equal(t, "Classic Clog Rosa", p.Name())
}

func TestGet_UnknownSlugIsNotFound(t *testing.T) {
	_, err := queries.GetHandler{Repository: &catalogtest.Repo{}}.Handle(context.Background(), "nao-existe")

	var nf httperr.NotFoundError
	assert.True(t, errors.As(err, &nf))
}

func TestList_AppliesDefaultAndMaxLimit(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		wantLimit int
	}{
		{"zero vira padrão", 0, ports.DefaultLimit},
		{"negativo vira padrão", -5, ports.DefaultLimit},
		{"acima do máximo é cortado", 1000, ports.MaxLimit},
		{"valor válido é mantido", 7, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &catalogtest.Repo{}
			_, err := queries.ListHandler{Repository: repo}.Handle(context.Background(), ports.Filter{Limit: tt.limit})
			require.NoError(t, err)
			assert.Equal(t, tt.wantLimit, repo.LastFilter.Limit)
		})
	}
}

func TestList_DefaultsSortToNewestAndNegativeOffsetToZero(t *testing.T) {
	repo := &catalogtest.Repo{}
	_, err := queries.ListHandler{Repository: repo}.Handle(context.Background(), ports.Filter{Offset: -3})

	require.NoError(t, err)
	assert.Equal(t, ports.SortNewest, repo.LastFilter.Sort)
	assert.Equal(t, 0, repo.LastFilter.Offset)
}
