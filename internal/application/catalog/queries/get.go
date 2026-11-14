package queries

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
)

type GetHandler struct {
	Repository ports.Repository
}

func (h GetHandler) Handle(ctx context.Context, slug string) (catalog.Product, error) {
	return h.Repository.GetBySlug(ctx, slug)
}
