package settings

import "time"

// Settings são os dados públicos da loja (contato e identidade), editáveis pelo painel.
// Contatos ficam vazios enquanto não forem configurados.
type Settings interface {
	Name() string
	City() string
	Description() string
	// WhatsApp e Phone guardam só os dígitos nacionais (DDD + número), sem o 55.
	WhatsApp() string
	Phone() string
	// Instagram é só o usuário, sem @ nem URL.
	Instagram() string
	UpdatedAt() time.Time
}

type settings struct {
	name, city, description string
	whatsapp, phone         string
	instagram               string
	updatedAt               time.Time
}

func (s *settings) Name() string         { return s.name }
func (s *settings) City() string         { return s.city }
func (s *settings) Description() string  { return s.description }
func (s *settings) WhatsApp() string     { return s.whatsapp }
func (s *settings) Phone() string        { return s.phone }
func (s *settings) Instagram() string    { return s.instagram }
func (s *settings) UpdatedAt() time.Time { return s.updatedAt }

// Snapshot são os dados já persistidos.
type Snapshot struct {
	Name, City, Description string
	WhatsApp, Phone         string
	Instagram               string
	UpdatedAt               time.Time
}

// Restore reconstrói as configurações vindas do banco, sem revalidar.
func Restore(s Snapshot) Settings {
	return &settings{
		name: s.Name, city: s.City, description: s.Description,
		whatsapp: s.WhatsApp, phone: s.Phone, instagram: s.Instagram, updatedAt: s.UpdatedAt,
	}
}

// InstagramURL monta o link do perfil; vazio se não houver usuário.
func InstagramURL(handle string) string {
	if handle == "" {
		return ""
	}
	return "https://www.instagram.com/" + handle
}
