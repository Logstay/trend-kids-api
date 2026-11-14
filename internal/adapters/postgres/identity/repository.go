package identity

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Logstay/trend-kids-api/internal/domain/identity"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/jmoiron/sqlx"
)

type Repository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

type row struct {
	UID          string `db:"uid"`
	Email        string `db:"email"`
	PasswordHash string `db:"password_hash"`
}

func (r *Repository) Save(ctx context.Context, a identity.Admin) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO admin_users (uid, email, password_hash) VALUES ($1, $2, $3)`,
		a.UID(), a.Email(), a.PasswordHash())
	return err
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (identity.Admin, error) {
	var rw row
	err := r.db.GetContext(ctx, &rw, `SELECT uid, email, password_hash FROM admin_users WHERE email = $1`, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httperr.NotFoundError("admin não encontrado")
	}
	if err != nil {
		return nil, err
	}
	return identity.Restore(rw.UID, rw.Email, rw.PasswordHash), nil
}

func (r *Repository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS (SELECT 1 FROM admin_users WHERE email = $1)`, email)
	return exists, err
}
