package commands

import (
	"context"
	goerrors "errors"
	"fmt"

	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

const maxCodeAttempts = 3

type PlaceHandler struct {
	Orders   ports.Repository
	Products ports.ProductReader
	Shipping orders.ShippingPolicy
}

// Handle cria o pedido. Preço, nome e disponibilidade vêm do catálogo; o estoque é baixado de
// forma atômica pelo repositório (as checagens daqui só dão uma mensagem amigável antecipada).
func (h PlaceHandler) Handle(ctx context.Context, cmd ports.PlaceOrder) (orders.Order, error) {
	for attempt := 0; attempt < maxCodeAttempts; attempt++ {
		order, err := h.build(ctx, cmd)
		if err != nil {
			return nil, err
		}

		err = h.Orders.Create(ctx, order)
		if goerrors.Is(err, ports.ErrCodeTaken) {
			continue // colisão rara de código: o build seguinte gera outro
		}
		if err != nil {
			return nil, err
		}
		return order, nil
	}
	return nil, httperr.InternalServerError("não foi possível gerar o código do pedido")
}

func (h PlaceHandler) build(ctx context.Context, cmd ports.PlaceOrder) (orders.Order, error) {
	f := orders.NewOrderFactory()
	if err := f.Customer(cmd.CustomerName, cmd.Phone, cmd.Neighborhood); err != nil {
		return nil, err
	}
	if len(cmd.Items) == 0 {
		return nil, httperr.RequiredError("o pedido precisa de ao menos um item")
	}

	requested := map[string]int{}
	for _, line := range cmd.Items {
		product, err := h.Products.GetByUID(ctx, line.ProductUID)
		if err != nil {
			return nil, err
		}
		if !product.Active() {
			return nil, httperr.ConflictError(fmt.Sprintf("produto indisponível: %s", product.Name()))
		}
		requested[line.ProductUID] += line.Quantity
		if requested[line.ProductUID] > product.Stock() {
			return nil, httperr.ConflictError(fmt.Sprintf("estoque insuficiente: %s", product.Name()))
		}
		if err := f.AddItem(product.UID(), product.Name(), product.PriceCents(), line.Quantity); err != nil {
			return nil, err
		}
	}
	return f.Build(h.Shipping)
}
