package sellers

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Logstay/trend-kids-api/internal/domain/sellers"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type Repository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

var (
	errNotFound  = httperr.NotFoundError("vendedora não encontrada")
	errDuplicate = httperr.ConflictError("esse WhatsApp já pertence a outra vendedora")
)

type row struct {
	UID       string    `db:"uid"`
	Name      string    `db:"name"`
	WhatsApp  string    `db:"whatsapp"`
	Active    bool      `db:"active"`
	CreatedAt time.Time `db:"created_at"`
}

func (r row) seller() sellers.Seller {
	return sellers.Restore(sellers.Snapshot{UID: r.UID, Name: r.Name, WhatsApp: r.WhatsApp, Active: r.Active, CreatedAt: r.CreatedAt})
}

func (r *Repository) Save(ctx context.Context, s sellers.Seller) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sellers (uid, name, whatsapp, active) VALUES ($1, $2, $3, $4)`,
		s.UID(), s.Name(), s.WhatsApp(), s.Active())
	return mapErr(err)
}

func (r *Repository) Update(ctx context.Context, s sellers.Seller) error {
	if _, err := uuid.Parse(s.UID()); err != nil {
		return errNotFound
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE sellers SET name = $2, whatsapp = $3, active = $4, updated_at = now() WHERE uid = $1`,
		s.UID(), s.Name(), s.WhatsApp(), s.Active())
	if err != nil {
		return mapErr(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, uid string) error {
	if _, err := uuid.Parse(uid); err != nil {
		return errNotFound
	}
	res, err := r.db.ExecContext(ctx, `DELETE FROM sellers WHERE uid = $1`, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	return nil
}

func (r *Repository) GetByUID(ctx context.Context, uid string) (sellers.Seller, error) {
	if _, err := uuid.Parse(uid); err != nil {
		return nil, errNotFound
	}
	var rw row
	err := r.db.GetContext(ctx, &rw, `SELECT uid, name, whatsapp, active, created_at FROM sellers WHERE uid = $1`, uid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return rw.seller(), nil
}

func (r *Repository) List(ctx context.Context, onlyActive bool) ([]sellers.Seller, error) {
	var rows []row
	err := r.db.SelectContext(ctx, &rows, `
		SELECT uid, name, whatsapp, active, created_at
		  FROM sellers
		 WHERE (NOT $1 OR active)
		 ORDER BY created_at, id`, onlyActive)
	if err != nil {
		return nil, err
	}
	out := make([]sellers.Seller, 0, len(rows))
	for _, rw := range rows {
		out = append(out, rw.seller())
	}
	return out, nil
}

// mapErr traduz a violação do índice único de WhatsApp (garantida pelo banco) em conflito.
func mapErr(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "sellers_whatsapp_key" {
		return errDuplicate
	}
	return err
}
