package ports

import (
	"context"
	"errors"
	"time"

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

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type ListFilter struct {
	Status string // vazio = todos
	Limit  int
	Offset int
}

// StatusEvent é uma linha do histórico. From vazio = criação do pedido.
type StatusEvent struct {
	From     orders.Status
	To       orders.Status
	AdminUID string // vazio quando o evento é do sistema (criação)
	At       time.Time
}

type ChangeStatusCommand struct {
	Code     string
	Status   string
	AdminUID string
}

// ChangeStatus é o que o repositório recebe, já com o status validado.
type ChangeStatus struct {
	Code     string
	To       orders.Status
	AdminUID string
}

// AdminOrder é o pedido com o seu histórico.
type AdminOrder struct {
	Order  orders.Order
	Events []StatusEvent
}

// AdminRepository é o que só o painel usa.
type AdminRepository interface {
	Repository

	// List ordena do mais novo para o mais antigo.
	List(ctx context.Context, f ListFilter) ([]orders.Order, error)
	Events(ctx context.Context, orderUID string) ([]StatusEvent, error)

	// UpdateStatus roda numa transação com a linha do pedido travada: valida a transição
	// (httperr.ConflictError se não for permitida), grava o evento e, ao cancelar, devolve o estoque.
	// Devolve httperr.NotFoundError se o código não existir.
	UpdateStatus(ctx context.Context, c ChangeStatus) (orders.Order, error)
}
