package commands

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
)

type UpdateHandler struct {
	Repository ports.Repository
}

func (h UpdateHandler) Handle(ctx context.Context, cmd ports.UpdateProduct) (catalog.Product, error) {
	current, err := h.Repository.GetByUID(ctx, cmd.UID)
	if err != nil {
		return nil, err
	}

	f := catalog.NewProductFactoryFrom(current)
	if err := f.Name(cmd.Name); err != nil {
		return nil, err
	}
	if err := f.Category(cmd.Category); err != nil {
		return nil, err
	}
	if err := f.PriceCents(cmd.PriceCents); err != nil {
		return nil, err
	}
	f.Description(cmd.Description)
	f.Active(cmd.Active)

	updated := f.Build()
	if err := h.Repository.Update(ctx, updated); err != nil {
		return nil, err
	}
	return updated, nil
}
