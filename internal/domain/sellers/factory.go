package sellers

import (
	"strings"
	"unicode/utf8"

	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/Logstay/trend-kids-api/pkg/phone"
	"github.com/google/uuid"
)

type SellerFactory interface {
	Name(name string) error
	WhatsApp(raw string) error
	Active(active bool)
	// Build falha se faltar nome ou WhatsApp.
	Build() (Seller, error)
}

type sellerFactory struct{ s *seller }

func NewSellerFactory() SellerFactory {
	return &sellerFactory{s: &seller{uid: uuid.NewString(), active: true}}
}

// NewSellerFactoryFrom parte de uma vendedora existente (edição): identidade e data de criação são mantidas.
func NewSellerFactoryFrom(s Seller) SellerFactory {
	copied := *(s.(*seller))
	return &sellerFactory{s: &copied}
}

const (
	minName = 2
	maxName = 60
)

func (f *sellerFactory) Name(name string) error {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < minName || n > maxName {
		return httperr.RequiredError("o nome da vendedora deve ter entre 2 e 60 caracteres")
	}
	f.s.name = name
	return nil
}

// WhatsApp é obrigatório: uma vendedora sem número não leva o cliente a lugar nenhum.
func (f *sellerFactory) WhatsApp(raw string) error {
	d, ok := phone.Normalize(raw)
	if !ok {
		return httperr.RequiredError("whatsapp inválido: informe DDD e número, ex.: (83) 99999-1234")
	}
	f.s.whatsapp = d
	return nil
}

func (f *sellerFactory) Active(active bool) { f.s.active = active }

func (f *sellerFactory) Build() (Seller, error) {
	if f.s.name == "" || f.s.whatsapp == "" {
		return nil, httperr.RequiredError("nome e whatsapp da vendedora são obrigatórios")
	}
	return f.s, nil
}
