// Package orderstest traz um repositório de pedidos em memória para testes dos casos de uso.
package orderstest

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type Repo struct {
	Orders    []orders.Order
	TakenFor  int   // as N primeiras chamadas a Create devolvem ErrCodeTaken
	CreateErr error // erro fixo em Create (ex.: estoque insuficiente)
	Calls     int
}

func (r *Repo) Create(_ context.Context, o orders.Order) error {
	r.Calls++
	if r.Calls <= r.TakenFor {
		return ports.ErrCodeTaken
	}
	if r.CreateErr != nil {
		return r.CreateErr
	}
	r.Orders = append(r.Orders, o)
	return nil
}

func (r *Repo) GetByCode(_ context.Context, code string) (orders.Order, error) {
	for _, o := range r.Orders {
		if o.Code() == code {
			return o, nil
		}
	}
	return nil, httperr.NotFoundError("pedido não encontrado")
}
