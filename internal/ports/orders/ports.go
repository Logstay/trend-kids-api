package ports

import (
	"context"
	"errors"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	"github.com/Logstay/trend-kids-api/internal/domain/orders"
)

// ErrCodeTaken: o código gerado já existe (colisão rara); o caso de uso gera outro e tenta de novo.
var ErrCodeTaken = errors.New("código de pedido já existe")

type Repository interface {
	// Create grava o pedido e baixa o estoque de cada item NA MESMA transação.
	// Devolve httperr.ConflictError("estoque insuficiente: <produto>") se algum item não tiver saldo;
	// nesse caso nada é gravado. Devolve ErrCodeTaken em colisão de código.
	Create(ctx context.Context, order orders.Order) error
	// GetByCode devolve httperr.NotFoundError se não existir.
	GetByCode(ctx context.Context, code string) (orders.Order, error)
}

// ProductReader é o pedaço do catálogo de que o pedido precisa (preço e disponibilidade).
type ProductReader interface {
	GetByUID(ctx context.Context, uid string) (catalog.Product, error)
}

type PlaceItem struct {
	ProductUID string
	Quantity   int
}

// PlaceOrder não tem preço de propósito: o preço sempre vem do catálogo, no servidor.
type PlaceOrder struct {
	CustomerName string
	Phone        string
	Neighborhood string
	Items        []PlaceItem
}

type Lookup struct {
	Code  string
	Phone string
}
