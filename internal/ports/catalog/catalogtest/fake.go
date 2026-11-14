// Package catalogtest traz um repositório em memória para testes dos casos de uso.
package catalogtest

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type Repo struct {
	Products   []catalog.Product
	Movements  []ports.StockMovement
	LastFilter ports.Filter
	Err        error
}

func (r *Repo) Save(_ context.Context, p catalog.Product) error {
	if r.Err != nil {
		return r.Err
	}
	r.Products = append(r.Products, p)
	return nil
}

func (r *Repo) GetBySlug(_ context.Context, slug string) (catalog.Product, error) {
	for _, p := range r.Products {
		if p.Slug() == slug {
			return p, nil
		}
	}
	return nil, httperr.NotFoundError("produto não encontrado")
}

func (r *Repo) ExistsBySlug(_ context.Context, slug string) (bool, error) {
	for _, p := range r.Products {
		if p.Slug() == slug {
			return true, nil
		}
	}
	return false, r.Err
}

func (r *Repo) List(_ context.Context, f ports.Filter) ([]catalog.Product, error) {
	r.LastFilter = f
	return r.Products, r.Err
}

func (r *Repo) GetByUID(_ context.Context, uid string) (catalog.Product, error) {
	for _, p := range r.Products {
		if p.UID() == uid {
			return p, nil
		}
	}
	return nil, httperr.NotFoundError("produto não encontrado")
}

func (r *Repo) Update(_ context.Context, updated catalog.Product) error {
	for i, p := range r.Products {
		if p.UID() == updated.UID() {
			r.Products[i] = updated
			return r.Err
		}
	}
	return httperr.NotFoundError("produto não encontrado")
}

func (r *Repo) AdjustStock(ctx context.Context, m ports.StockMovement) (int, error) {
	p, err := r.GetByUID(ctx, m.ProductUID)
	if err != nil {
		return 0, err
	}
	if p.Stock()+m.Delta < 0 {
		return 0, httperr.ConflictError("estoque insuficiente")
	}
	f := catalog.NewProductFactoryFrom(p)
	_ = f.Stock(p.Stock() + m.Delta)
	_ = r.Update(ctx, f.Build())
	r.Movements = append(r.Movements, m)
	return p.Stock() + m.Delta, nil
}
