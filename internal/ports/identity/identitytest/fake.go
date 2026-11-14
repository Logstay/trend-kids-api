// Package identitytest traz um repositório de admins em memória para testes.
package identitytest

import (
	"context"

	"github.com/Logstay/trend-kids-api/internal/domain/identity"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
)

type Repo struct{ Admins []identity.Admin }

func (r *Repo) Save(_ context.Context, a identity.Admin) error {
	r.Admins = append(r.Admins, a)
	return nil
}

func (r *Repo) FindByEmail(_ context.Context, email string) (identity.Admin, error) {
	for _, a := range r.Admins {
		if a.Email() == email {
			return a, nil
		}
	}
	return nil, httperr.NotFoundError("admin não encontrado")
}

func (r *Repo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	_, err := r.FindByEmail(ctx, email)
	return err == nil, nil
}
