package commands

import (
	"context"
	"strings"

	"github.com/Logstay/trend-kids-api/internal/domain/identity"
	ports "github.com/Logstay/trend-kids-api/internal/ports/identity"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type CreateAdminHandler struct {
	Repository ports.AdminRepository
	Hasher     ports.PasswordHasher
}

func (h CreateAdminHandler) Handle(ctx context.Context, cmd ports.CreateAdminCommand) error {
	if len([]rune(cmd.Password)) < ports.MinPasswordLen {
		return httperr.RequiredError("a senha precisa ter ao menos 12 caracteres")
	}

	f := identity.NewAdminFactory()
	if err := f.Email(cmd.Email); err != nil {
		return err
	}
	email := f.Build().Email()

	exists, err := h.Repository.ExistsByEmail(ctx, email)
	if err != nil {
		return err
	}
	if exists {
		return httperr.ConflictError("já existe admin com esse e-mail")
	}

	hash, err := h.Hasher.Hash(cmd.Password)
	if err != nil {
		return err
	}
	if err := f.PasswordHash(hash); err != nil {
		return err
	}
	return h.Repository.Save(ctx, f.Build())
}

func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
