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
