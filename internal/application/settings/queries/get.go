package queries

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/settings"
	ports "github.com/Logstay/trend-kids-api/internal/ports/settings"
)

type GetHandler struct {
	Repository ports.Repository
}

func (h GetHandler) Handle(ctx context.Context) (settings.Settings, error) {
	return h.Repository.Get(ctx)
}
