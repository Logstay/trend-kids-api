package queries

import (
	"context"
	"strings"

	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type AdminListHandler struct {
	Orders ports.AdminRepository
}

func (h AdminListHandler) Handle(ctx context.Context, f ports.ListFilter) ([]orders.Order, error) {
	if f.Status != "" {
		if _, ok := orders.ParseStatus(f.Status); !ok {
			return nil, httperr.BadRequestError("status inválido")
		}
	}
	if f.Limit <= 0 {
		f.Limit = ports.DefaultLimit
	}
	if f.Limit > ports.MaxLimit {
		f.Limit = ports.MaxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	return h.Orders.List(ctx, f)
}

type AdminGetHandler struct {
	Orders ports.AdminRepository
}

func (h AdminGetHandler) Handle(ctx context.Context, code string) (ports.AdminOrder, error) {
	order, err := h.Orders.GetByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return ports.AdminOrder{}, err
	}
	events, err := h.Orders.Events(ctx, order.UID())
	if err != nil {
		return ports.AdminOrder{}, err
	}
	return ports.AdminOrder{Order: order, Events: events}, nil
}
