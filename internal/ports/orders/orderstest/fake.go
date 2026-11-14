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

	LastFilter ports.ListFilter
	Changes    []ports.ChangeStatus
	EventLog   map[string][]ports.StatusEvent
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

// ---- lado admin ----

func (r *Repo) List(_ context.Context, f ports.ListFilter) ([]orders.Order, error) {
	r.LastFilter = f
	out := make([]orders.Order, 0, len(r.Orders))
	for _, o := range r.Orders {
		if f.Status == "" || string(o.Status()) == f.Status {
			out = append(out, o)
		}
	}
	return out, nil
}

func (r *Repo) Events(_ context.Context, orderUID string) ([]ports.StatusEvent, error) {
	return r.EventLog[orderUID], nil
}

// UpdateStatus imita a regra do repositório real (a regra de verdade é testada contra o Postgres).
func (r *Repo) UpdateStatus(_ context.Context, c ports.ChangeStatus) (orders.Order, error) {
	r.Changes = append(r.Changes, c)
	for i, o := range r.Orders {
		if o.Code() != c.Code {
			continue
		}
		if !orders.CanTransition(o.Status(), c.To) {
			return nil, httperr.ConflictError("transição de status não permitida: " + string(o.Status()) + " → " + string(c.To))
		}
		updated := orders.Restore(orders.Snapshot{
			UID: o.UID(), Code: o.Code(), CustomerName: o.CustomerName(), Phone: o.CustomerPhone(),
			Neighborhood: o.Neighborhood(), Status: c.To, Items: o.Items(),
			SubtotalCents: o.SubtotalCents(), ShippingCents: o.ShippingCents(), TotalCents: o.TotalCents(), CreatedAt: o.CreatedAt(),
		})
		r.Orders[i] = updated
		if r.EventLog == nil {
			r.EventLog = map[string][]ports.StatusEvent{}
		}
		r.EventLog[o.UID()] = append(r.EventLog[o.UID()], ports.StatusEvent{From: o.Status(), To: c.To, AdminUID: c.AdminUID})
		return updated, nil
	}
	return nil, httperr.NotFoundError("pedido não encontrado")
}
