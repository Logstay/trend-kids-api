package orders

import (
	"crypto/rand"
	"math/big"
	"strings"
	"unicode/utf8"

	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/Logstay/trend-kids-api/pkg/phone"
	"github.com/google/uuid"
)

type OrderFactory interface {
	Customer(name, phone, neighborhood string) error
	AddItem(productUID, name string, unitPriceCents int64, quantity int) error
	Build(policy ShippingPolicy) (Order, error)
}

type orderFactory struct {
	order *order
}

func NewOrderFactory() OrderFactory {
	return &orderFactory{order: &order{uid: uuid.NewString(), code: newCode(), status: StatusNew}}
}

const (
	minNameLen, maxNameLen       = 2, 100
	minHoodLen, maxHoodLen       = 2, 80
	maxUnitPriceCents      int64 = 100_000_000
)

func (f *orderFactory) Customer(name, phone, neighborhood string) error {
	name, neighborhood = strings.TrimSpace(name), strings.TrimSpace(neighborhood)
	if n := utf8.RuneCountInString(name); n < minNameLen || n > maxNameLen {
		return httperr.RequiredError("nome do cliente deve ter entre 2 e 100 caracteres")
	}
	if n := utf8.RuneCountInString(neighborhood); n < minHoodLen || n > maxHoodLen {
		return httperr.RequiredError("bairro deve ter entre 2 e 80 caracteres")
	}
	digits, ok := NormalizePhone(phone)
	if !ok {
		return httperr.RequiredError("telefone inválido: informe DDD e número, ex.: (83) 99999-1234")
	}
	f.order.customerName, f.order.neighborhood, f.order.phone = name, neighborhood, digits
	return nil
}

// NormalizePhone devolve só os dígitos nacionais (DDD + número), sem o 55.
func NormalizePhone(raw string) (string, bool) { return phone.Normalize(raw) }

func (f *orderFactory) AddItem(productUID, name string, unitPriceCents int64, quantity int) error {
	name = strings.TrimSpace(name)
	switch {
	case productUID == "":
		return httperr.RequiredError("produto do item é obrigatório")
	case name == "":
		return httperr.RequiredError("nome do produto é obrigatório")
	case unitPriceCents <= 0 || unitPriceCents > maxUnitPriceCents:
		return httperr.RequiredError("preço do item inválido")
	case quantity < 1:
		return httperr.RequiredError("quantidade deve ser ao menos 1")
	}

	for i, it := range f.order.items {
		if it.ProductUID == productUID {
			if it.Quantity+quantity > MaxQuantity {
				return httperr.RequiredError("quantidade máxima por produto é 20")
			}
			f.order.items[i].Quantity += quantity
			return nil
		}
	}
	if quantity > MaxQuantity {
		return httperr.RequiredError("quantidade máxima por produto é 20")
	}
	if len(f.order.items) >= MaxLines {
		return httperr.RequiredError("máximo de 20 produtos diferentes por pedido")
	}
	f.order.items = append(f.order.items, Item{ProductUID: productUID, Name: name, UnitPriceCents: unitPriceCents, Quantity: quantity})
	return nil
}

func (f *orderFactory) Build(policy ShippingPolicy) (Order, error) {
	o := f.order
	if o.customerName == "" {
		return nil, httperr.RequiredError("dados do cliente são obrigatórios")
	}
	if len(o.items) == 0 {
		return nil, httperr.RequiredError("o pedido precisa de ao menos um item")
	}
	o.subtotal = 0
	for _, it := range o.items {
		o.subtotal += it.TotalCents()
	}
	o.shipping = policy.CostFor(o.subtotal)
	o.total = o.subtotal + o.shipping
	return o, nil
}

// Alfabeto sem 0/O, 1/I/L: o código é ditado por telefone e digitado no WhatsApp.
const codeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

func newCode() string {
	max := big.NewInt(int64(len(codeAlphabet)))
	b := make([]byte, 8)
	for i := range b {
		n, err := rand.Int(rand.Reader, max) // sem viés de módulo
		if err != nil {
			panic("crypto/rand indisponível: " + err.Error())
		}
		b[i] = codeAlphabet[n.Int64()]
	}
	return "TK" + string(b)
}
