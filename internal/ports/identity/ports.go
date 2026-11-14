package ports

import (
	"context"
	"time"

	"github.com/Logstay/trend-kids-api/internal/domain/identity"
)

// AdminRepository: FindByEmail devolve httperr.NotFoundError quando não existe.
type AdminRepository interface {
	Save(ctx context.Context, admin identity.Admin) error
	FindByEmail(ctx context.Context, email string) (identity.Admin, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, hash string) bool
}

type TokenService interface {
	Issue(adminUID string) (token string, expiresAt time.Time, err error)
	Parse(token string) (adminUID string, err error)
}

type LoginCommand struct {
	Email    string
	Password string
}

type Session struct {
	Token     string
	ExpiresAt time.Time
}

type CreateAdminCommand struct {
	Email    string
	Password string
}

const MinPasswordLen = 12
