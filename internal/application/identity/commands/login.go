package commands

import (
	"context"
	goerrors "errors"

	ports "github.com/Logstay/trend-kids-api/internal/ports/identity"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type LoginHandler struct {
	Repository ports.AdminRepository
	Hasher     ports.PasswordHasher
	Tokens     ports.TokenService
}

var errInvalidCredentials = httperr.UnauthorizedError("e-mail ou senha inválidos")

func (h LoginHandler) Handle(ctx context.Context, cmd ports.LoginCommand) (ports.Session, error) {
	admin, err := h.Repository.FindByEmail(ctx, normalizeEmail(cmd.Email))
	if err != nil {
		var notFound httperr.NotFoundError
		if goerrors.As(err, &notFound) {
			// Gasta o mesmo tempo de um login real, para o tempo de resposta não revelar se o e-mail existe.
			_, _ = h.Hasher.Hash(cmd.Password)
			return ports.Session{}, errInvalidCredentials
		}
		return ports.Session{}, err
	}

	if !h.Hasher.Verify(cmd.Password, admin.PasswordHash()) {
		return ports.Session{}, errInvalidCredentials
	}

	token, expires, err := h.Tokens.Issue(admin.UID())
	if err != nil {
		return ports.Session{}, err
	}
	return ports.Session{Token: token, ExpiresAt: expires}, nil
}
