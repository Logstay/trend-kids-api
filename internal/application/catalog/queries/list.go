package queries

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
)

type ListHandler struct {
	Repository ports.Repository
}

func (h ListHandler) Handle(ctx context.Context, filter ports.Filter) ([]catalog.Product, error) {
	if filter.Limit <= 0 {
		filter.Limit = ports.DefaultLimit
	}
	if filter.Limit > ports.MaxLimit {
		filter.Limit = ports.MaxLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	if filter.Sort == "" {
		filter.Sort = ports.SortNewest
	}
	return h.Repository.List(ctx, filter)
}
