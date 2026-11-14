package identity

import (
	"net/mail"
	"strings"

	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/google/uuid"
)

type AdminFactory interface {
	Email(email string) error
	PasswordHash(hash string) error
	Build() Admin
}

type adminFactory struct{ admin *admin }

func NewAdminFactory() AdminFactory {
	return &adminFactory{admin: &admin{uid: uuid.NewString()}}
}

func (f *adminFactory) Email(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	addr, err := mail.ParseAddress(email)
	// ParseAddress aceita "Nome <a@b.com>"; exigimos só o endereço puro.
	if err != nil || addr.Address != email {
		return httperr.RequiredError("e-mail inválido")
	}
	f.admin.email = email
	return nil
}

func (f *adminFactory) PasswordHash(hash string) error {
	if hash == "" {
		return httperr.RequiredError("hash da senha é obrigatório")
	}
	f.admin.passwordHash = hash
	return nil
}

func (f *adminFactory) Build() Admin { return f.admin }
