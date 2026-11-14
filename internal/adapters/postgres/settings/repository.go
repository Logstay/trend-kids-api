package settings

import (
	"context"
	"time"

	"github.com/Logstay/trend-kids-api/internal/domain/settings"
	"github.com/jmoiron/sqlx"
)

type Repository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

type row struct {
	Name        string    `db:"name"`
	City        string    `db:"city"`
	Description string    `db:"description"`
	WhatsApp    string    `db:"whatsapp"`
	Phone       string    `db:"phone"`
	Instagram   string    `db:"instagram"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// Get lê a linha única (criada pela migração com valores padrão).
func (r *Repository) Get(ctx context.Context) (settings.Settings, error) {
	var rw row
	err := r.db.GetContext(ctx, &rw, `
		SELECT name, city, description, whatsapp, phone, instagram, updated_at
		  FROM store_settings WHERE id = 1`)
	if err != nil {
		return nil, err
	}
	return settings.Restore(settings.Snapshot{
		Name: rw.Name, City: rw.City, Description: rw.Description,
		WhatsApp: rw.WhatsApp, Phone: rw.Phone, Instagram: rw.Instagram, UpdatedAt: rw.UpdatedAt,
	}), nil
}

// Update é um upsert na linha única: funciona mesmo se alguém tiver apagado a linha padrão.
func (r *Repository) Update(ctx context.Context, s settings.Settings, adminUID string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO store_settings (id, name, city, description, whatsapp, phone, instagram, updated_by, updated_at)
		VALUES (1, $1, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid, now())
		ON CONFLICT (id) DO UPDATE
		   SET name = EXCLUDED.name, city = EXCLUDED.city, description = EXCLUDED.description,
		       whatsapp = EXCLUDED.whatsapp, phone = EXCLUDED.phone, instagram = EXCLUDED.instagram,
		       updated_by = EXCLUDED.updated_by, updated_at = now()`,
		s.Name(), s.City(), s.Description(), s.WhatsApp(), s.Phone(), s.Instagram(), adminUID)
	return err
}
