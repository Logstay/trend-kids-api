package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Logstay/trend-kids-api/internal/domain/catalog"
	ports "github.com/Logstay/trend-kids-api/internal/ports/catalog"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/jmoiron/sqlx"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

type row struct {
	UID         string    `db:"uid"`
	Name        string    `db:"name"`
	Slug        string    `db:"slug"`
	Category    string    `db:"category"`
	Description string    `db:"description"`
	PriceCents  int64     `db:"price_cents"`
	Stock       int       `db:"stock"`
	Active      bool      `db:"active"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (r row) product() catalog.Product {
	return catalog.Restore(catalog.Snapshot{
		UID: r.UID, Name: r.Name, Slug: r.Slug, Category: r.Category, Description: r.Description,
		PriceCents: r.PriceCents, Stock: r.Stock, Active: r.Active, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

const columns = `uid, name, slug, category, description, price_cents, stock, active, created_at, updated_at`

func (r *Repository) Save(ctx context.Context, p catalog.Product) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO products (uid, name, slug, category, description, price_cents, stock, active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		p.UID(), p.Name(), p.Slug(), string(p.Category()), p.Description(), p.PriceCents(), p.Stock(), p.Active())
	return err
}

func (r *Repository) GetBySlug(ctx context.Context, slug string) (catalog.Product, error) {
	var rw row
	err := r.db.GetContext(ctx, &rw, `SELECT `+columns+` FROM products WHERE slug = $1 AND active`, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httperr.NotFoundError("produto não encontrado")
	}
	if err != nil {
		return nil, err
	}
	return rw.product(), nil
}

func (r *Repository) ExistsBySlug(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS (SELECT 1 FROM products WHERE slug = $1)`, slug)
	return exists, err
}

var orderBy = map[ports.Sort]string{
	ports.SortNewest:    "created_at DESC, id DESC",
	ports.SortPriceAsc:  "price_cents ASC, id ASC",
	ports.SortPriceDesc: "price_cents DESC, id ASC",
}

func (r *Repository) List(ctx context.Context, f ports.Filter) ([]catalog.Product, error) {
	where := []string{"active"}
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}

	if f.Category != "" {
		add("category = $%d", f.Category)
	}
	if f.Query != "" {
		add(`unaccent(name) ILIKE '%%' || unaccent($%d) || '%%'`, escapeLike(f.Query))
	}

	order, ok := orderBy[f.Sort]
	if !ok {
		order = orderBy[ports.SortNewest]
	}
	args = append(args, f.Limit, f.Offset)
	query := fmt.Sprintf(`SELECT %s FROM products WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		columns, strings.Join(where, " AND "), order, len(args)-1, len(args))

	var rows []row
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	result := make([]catalog.Product, 0, len(rows))
	for _, rw := range rows {
		result = append(result, rw.product())
	}
	return result, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
