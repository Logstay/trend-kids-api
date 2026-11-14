package orders

import "time"

type Status string

const (
	StatusNew       Status = "new"
	StatusPaid      Status = "paid"
	StatusShipped   Status = "shipped"
	StatusDelivered Status = "delivered"
	StatusCancelled Status = "cancelled"
)

const (
	MaxQuantity = 20 // por linha
	MaxLines    = 20 // produtos distintos por pedido
)

// Item é a linha do pedido com SNAPSHOT: nome e preço no momento da compra.
// O catálogo muda depois; o pedido não.
type Item struct {
	ProductUID     string
	Name           string
	UnitPriceCents int64
	Quantity       int
}

func (i Item) TotalCents() int64 { return i.UnitPriceCents * int64(i.Quantity) }

type Order interface {
	UID() string
	Code() string
	CustomerName() string
	CustomerPhone() string
	Neighborhood() string
	Status() Status
	Items() []Item
	SubtotalCents() int64
	ShippingCents() int64
	TotalCents() int64
	CreatedAt() time.Time
}

type order struct {
	uid, code                         string
	customerName, phone, neighborhood string
	status                            Status
	items                             []Item
	subtotal, shipping, total         int64
	createdAt                         time.Time
}

func (o *order) UID() string           { return o.uid }
func (o *order) Code() string          { return o.code }
func (o *order) CustomerName() string  { return o.customerName }
func (o *order) CustomerPhone() string { return o.phone }
func (o *order) Neighborhood() string  { return o.neighborhood }
func (o *order) Status() Status        { return o.status }
func (o *order) SubtotalCents() int64  { return o.subtotal }
func (o *order) ShippingCents() int64  { return o.shipping }
func (o *order) TotalCents() int64     { return o.total }
func (o *order) CreatedAt() time.Time  { return o.createdAt }
func (o *order) Items() []Item         { return append([]Item(nil), o.items...) }

// Snapshot são os dados de um pedido já persistido.
type Snapshot struct {
	UID, Code                         string
	CustomerName, Phone, Neighborhood string
	Status                            Status
	Items                             []Item
	SubtotalCents, ShippingCents      int64
	TotalCents                        int64
	CreatedAt                         time.Time
}

// Restore reconstrói um pedido vindo do banco, sem recalcular nada.
func Restore(s Snapshot) Order {
	return &order{
		uid: s.UID, code: s.Code, customerName: s.CustomerName, phone: s.Phone, neighborhood: s.Neighborhood,
		status: s.Status, items: append([]Item(nil), s.Items...),
		subtotal: s.SubtotalCents, shipping: s.ShippingCents, total: s.TotalCents, createdAt: s.CreatedAt,
	}
}

// ShippingPolicy: frete fixo, grátis a partir de um valor. Tudo zero = sem frete
// (a regra real da loja ainda é uma decisão em aberto).
type ShippingPolicy struct {
	FlatCents      int64
	FreeAboveCents int64 // 0 = nunca é grátis por valor
}

func (p ShippingPolicy) CostFor(subtotalCents int64) int64 {
	if p.FreeAboveCents > 0 && subtotalCents >= p.FreeAboveCents {
		return 0
	}
	return p.FlatCents
}
