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

func (h SaveHandler) Handle(ctx context.Context, cmd ports.SaveProduct) (catalog.Product, error) {
	product, err := build(cmd)
	if err != nil {
		return nil, err
	}

	exists, err := h.Repository.ExistsBySlug(ctx, product.Slug())
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, httperr.ConflictError("já existe produto com esse nome")
	}

	if err := h.Repository.Save(ctx, product); err != nil {
		return nil, err
	}
	// Relê para devolver também os campos preenchidos pelo banco (datas).
	return h.Repository.GetByUID(ctx, product.UID())
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
