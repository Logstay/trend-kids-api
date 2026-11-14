package queries

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/sellers"
	ports "github.com/Logstay/trend-kids-api/internal/ports/sellers"
)

type ListHandler struct {
	Repository ports.Repository
}

// Handle: onlyActive=true é a visão pública; false é a do painel.
func (h ListHandler) Handle(ctx context.Context, onlyActive bool) ([]sellers.Seller, error) {
	return h.Repository.List(ctx, onlyActive)
}
