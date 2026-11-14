// Package sellerstest traz um repositório de vendedoras em memória para testes dos casos de uso.
package sellerstest

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/sellers"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type Repo struct {
	Sellers    []sellers.Seller
	LastActive bool // último valor de onlyActive recebido em List
}

func (r *Repo) dup(s sellers.Seller) bool {
	for _, o := range r.Sellers {
		if o.WhatsApp() == s.WhatsApp() && o.UID() != s.UID() {
			return true
		}
	}
	return false
}

func (r *Repo) Save(_ context.Context, s sellers.Seller) error {
	if r.dup(s) {
		return httperr.ConflictError("esse WhatsApp já pertence a outra vendedora")
	}
	r.Sellers = append(r.Sellers, s)
	return nil
}

func (r *Repo) Update(_ context.Context, s sellers.Seller) error {
	if r.dup(s) {
		return httperr.ConflictError("esse WhatsApp já pertence a outra vendedora")
	}
	for i, o := range r.Sellers {
		if o.UID() == s.UID() {
			r.Sellers[i] = s
			return nil
		}
	}
	return httperr.NotFoundError("vendedora não encontrada")
}

func (r *Repo) Delete(_ context.Context, uid string) error {
	for i, o := range r.Sellers {
		if o.UID() == uid {
			r.Sellers = append(r.Sellers[:i], r.Sellers[i+1:]...)
			return nil
		}
	}
	return httperr.NotFoundError("vendedora não encontrada")
}

func (r *Repo) GetByUID(_ context.Context, uid string) (sellers.Seller, error) {
	for _, o := range r.Sellers {
		if o.UID() == uid {
			return o, nil
		}
	}
	return nil, httperr.NotFoundError("vendedora não encontrada")
}

func (r *Repo) List(_ context.Context, onlyActive bool) ([]sellers.Seller, error) {
	r.LastActive = onlyActive
	out := make([]sellers.Seller, 0, len(r.Sellers))
	for _, s := range r.Sellers {
		if !onlyActive || s.Active() {
			out = append(out, s)
		}
	}
	return out, nil
}
