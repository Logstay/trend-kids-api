package sellers

import "time"

// Seller é uma vendedora que atende pelo WhatsApp. É para o número dela que o cliente é levado
// quando escolhe com quem quer falar. O WhatsApp de contato geral da loja fica em Configurações.
type Seller interface {
	UID() string
	Name() string
	// WhatsApp guarda só os dígitos nacionais (DDD + número), sem o 55.
	WhatsApp() string
	Active() bool
	CreatedAt() time.Time
}

type seller struct {
	uid       string
	name      string
	whatsapp  string
	active    bool
	createdAt time.Time
}

func (s *seller) UID() string          { return s.uid }
func (s *seller) Name() string         { return s.name }
func (s *seller) WhatsApp() string     { return s.whatsapp }
func (s *seller) Active() bool         { return s.active }
func (s *seller) CreatedAt() time.Time { return s.createdAt }

// Snapshot são os dados já persistidos.
type Snapshot struct {
	UID       string
	Name      string
	WhatsApp  string
	Active    bool
	CreatedAt time.Time
}

// Restore reconstrói uma vendedora vinda do banco, sem revalidar.
func Restore(s Snapshot) Seller {
	return &seller{uid: s.UID, name: s.Name, whatsapp: s.WhatsApp, active: s.Active, createdAt: s.CreatedAt}
}
