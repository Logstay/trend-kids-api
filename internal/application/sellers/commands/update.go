package commands

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/sellers"
	ports "github.com/Logstay/trend-kids-api/internal/ports/sellers"
)

type UpdateHandler struct {
	Repository ports.Repository
}

func (h UpdateHandler) Handle(ctx context.Context, cmd ports.UpdateSeller) (sellers.Seller, error) {
	current, err := h.Repository.GetByUID(ctx, cmd.UID)
	if err != nil {
		return nil, err
	}

	f := sellers.NewSellerFactoryFrom(current)
	if err := f.Name(cmd.Name); err != nil {
		return nil, err
	}
	if err := f.WhatsApp(cmd.WhatsApp); err != nil {
		return nil, err
	}
	f.Active(cmd.Active)
	built, err := f.Build()
	if err != nil {
		return nil, err
	}

	if err := h.Repository.Update(ctx, built); err != nil {
		return nil, err
	}
	return h.Repository.GetByUID(ctx, cmd.UID)
}
