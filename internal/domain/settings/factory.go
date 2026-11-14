package settings

import (
	"regexp"
	"strings"
	"unicode/utf8"

	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/Logstay/trend-kids-api/pkg/phone"
)

type SettingsFactory interface {
	Name(name string) error
	City(city string) error
	Description(description string) error
	WhatsApp(raw string) error
	Phone(raw string) error
	Instagram(raw string) error
	// Build falha se faltar nome ou cidade.
	Build() (Settings, error)
}

type settingsFactory struct{ s *settings }

func NewSettingsFactory() SettingsFactory { return &settingsFactory{s: &settings{}} }

const (
	minShort, maxShort = 2, 80
	maxDescription     = 300
	maxHandle          = 30
)

func (f *settingsFactory) Name(name string) error {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < minShort || n > maxShort {
		return httperr.RequiredError("o nome da loja deve ter entre 2 e 80 caracteres")
	}
	f.s.name = name
	return nil
}

func (f *settingsFactory) City(city string) error {
	city = strings.TrimSpace(city)
	if n := utf8.RuneCountInString(city); n < minShort || n > maxShort {
		return httperr.RequiredError("a cidade deve ter entre 2 e 80 caracteres")
	}
	f.s.city = city
	return nil
}

func (f *settingsFactory) Description(description string) error {
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > maxDescription {
		return httperr.RequiredError("a descrição pode ter no máximo 300 caracteres")
	}
	f.s.description = description
	return nil
}

func (f *settingsFactory) WhatsApp(raw string) error {
	d, err := contact(raw, "whatsapp")
	f.s.whatsapp = d
	return err
}

func (f *settingsFactory) Phone(raw string) error {
	d, err := contact(raw, "telefone")
	f.s.phone = d
	return err
}

// contact: vazio = não configurado; senão precisa ser um telefone brasileiro válido.
func contact(raw, label string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	d, ok := phone.Normalize(raw)
	if !ok {
		return "", httperr.RequiredError(label + " inválido: informe DDD e número, ex.: (83) 99999-1234")
	}
	return d, nil
}

var handleRe = regexp.MustCompile(`^[A-Za-z0-9._]{1,30}$`)

// Instagram aceita "@usuario", "usuario" ou a URL do perfil, e guarda só o usuário em minúsculas.
func (f *settingsFactory) Instagram(raw string) error {
	h := strings.TrimSpace(raw)
	if h == "" {
		f.s.instagram = ""
		return nil
	}

	lower := strings.ToLower(h)
	for _, prefix := range []string{"https://", "http://"} {
		lower = strings.TrimPrefix(lower, prefix)
	}
	if strings.Contains(lower, "/") || strings.Contains(lower, "?") || strings.Contains(lower, ".com") && strings.Contains(lower, "instagram") {
		// parece URL: só vale instagram.com/<usuario>
		lower = strings.TrimPrefix(lower, "www.")
		if !strings.HasPrefix(lower, "instagram.com/") {
			return invalidHandle()
		}
		lower = strings.TrimPrefix(lower, "instagram.com/")
		if i := strings.IndexAny(lower, "/?#"); i >= 0 {
			lower = lower[:i]
		}
	}
	lower = strings.TrimPrefix(lower, "@")

	if !handleRe.MatchString(lower) || strings.HasPrefix(lower, "@") {
		return invalidHandle()
	}
	f.s.instagram = lower
	return nil
}

func invalidHandle() error {
	return httperr.RequiredError("instagram inválido: use @usuario ou o link do perfil")
}

func (f *settingsFactory) Build() (Settings, error) {
	if f.s.name == "" || f.s.city == "" {
		return nil, httperr.RequiredError("nome e cidade da loja são obrigatórios")
	}
	return f.s, nil
}
