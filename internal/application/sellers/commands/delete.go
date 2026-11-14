package commands

import (
	"context"

	ports "github.com/Logstay/trend-kids-api/internal/ports/sellers"
)

type DeleteHandler struct {
	Repository ports.Repository
}

func (h DeleteHandler) Handle(ctx context.Context, uid string) error {
	return h.Repository.Delete(ctx, uid)
}
