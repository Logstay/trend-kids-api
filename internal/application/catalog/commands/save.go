package commands

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type SaveHandler struct {
	Repository ports.Repository
}

func (h SaveHandler) Handle(ctx context.Context, cmd ports.SaveProduct) error {
	product, err := build(cmd)
	if err != nil {
		return err
	}

	exists, err := h.Repository.ExistsBySlug(ctx, product.Slug())
	if err != nil {
		return err
	}
	if exists {
		return httperr.ConflictError("já existe produto com esse nome")
	}

	return h.Repository.Save(ctx, product)
}

func build(cmd ports.SaveProduct) (catalog.Product, error) {
	f := catalog.NewProductFactory()
	if err := f.Name(cmd.Name); err != nil {
		return nil, err
	}
	if err := f.Category(cmd.Category); err != nil {
		return nil, err
	}
	if err := f.PriceCents(cmd.PriceCents); err != nil {
		return nil, err
	}
	if err := f.Stock(cmd.Stock); err != nil {
		return nil, err
	}
	f.Description(cmd.Description)
	return f.Build(), nil
}
