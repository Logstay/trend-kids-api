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

func (r *Repo) withImages(i int, images []string) {
	p := r.Products[i]
	r.Products[i] = catalog.Restore(catalog.Snapshot{
		UID: p.UID(), Name: p.Name(), Slug: p.Slug(), Category: string(p.Category()), Description: p.Description(),
		PriceCents: p.PriceCents(), Stock: p.Stock(), Active: p.Active(), Images: images, CreatedAt: p.CreatedAt(), UpdatedAt: p.UpdatedAt(),
	})
}

func (r *Repo) index(uid string) int {
	for i, p := range r.Products {
		if p.UID() == uid {
			return i
		}
	}
	return -1
}

func (r *Repo) AddImage(_ context.Context, uid, key string) error {
	i := r.index(uid)
	if i < 0 {
		return httperr.NotFoundError("produto não encontrado")
	}
	current := r.Products[i].Images()
	for _, k := range current {
		if k == key {
			return httperr.ConflictError("essa foto já está cadastrada")
		}
	}
	if len(current) >= ports.MaxImages {
		return httperr.ConflictError("limite de fotos por produto atingido")
	}
	if r.Err != nil {
		return r.Err
	}
	r.withImages(i, append(current, key))
	return nil
}

func (r *Repo) RemoveImage(_ context.Context, uid, key string) error {
	i := r.index(uid)
	if i < 0 {
		return httperr.NotFoundError("produto não encontrado")
	}
	var kept []string
	found := false
	for _, k := range r.Products[i].Images() {
		if k == key {
			found = true
			continue
		}
		kept = append(kept, k)
	}
	if !found {
		return httperr.NotFoundError("foto não encontrada")
	}
	if r.Err != nil {
		return r.Err
	}
	r.withImages(i, kept)
	return nil
}

func (r *Repo) ReorderImages(_ context.Context, uid string, keys []string) error {
	i := r.index(uid)
	if i < 0 {
		return httperr.NotFoundError("produto não encontrado")
	}
	current := r.Products[i].Images()
	seen := map[string]bool{}
	for _, k := range keys {
		seen[k] = true
	}
	valid := len(keys) == len(current) && len(seen) == len(keys)
	for _, k := range current {
		valid = valid && seen[k]
	}
	if !valid {
		return httperr.RequiredError("a nova ordem precisa ter exatamente as fotos atuais, sem repetir")
	}
	if r.Err != nil {
		return r.Err
	}
	r.withImages(i, append([]string(nil), keys...))
	return nil
}
