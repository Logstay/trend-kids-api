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
	"github.com/google/uuid"
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

const initialStockReason = "estoque inicial"

// Save grava o produto e, se houver estoque inicial, a primeira movimentação (o livro-razão sempre fecha com o saldo).
func (r *Repository) Save(ctx context.Context, p catalog.Product) error {
	return r.inTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO products (uid, name, slug, category, description, price_cents, stock, active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			p.UID(), p.Name(), p.Slug(), string(p.Category()), p.Description(), p.PriceCents(), p.Stock(), p.Active())
		if err != nil || p.Stock() == 0 {
			return err
		}
		return insertMovement(ctx, tx, ports.StockMovement{ProductUID: p.UID(), Delta: p.Stock(), Reason: initialStockReason})
	})
}

func (r *Repository) GetByUID(ctx context.Context, uid string) (catalog.Product, error) {
	if _, err := uuid.Parse(uid); err != nil {
		return nil, errProductNotFound
	}
	var rw row
	err := r.db.GetContext(ctx, &rw, `SELECT `+columns+` FROM products WHERE uid = $1`, uid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errProductNotFound
	}
	if err != nil {
		return nil, err
	}
	return rw.product(), nil
}

// Update não toca em estoque nem em slug: o primeiro só muda por movimentação, o segundo é a URL pública.
func (r *Repository) Update(ctx context.Context, p catalog.Product) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE products
		   SET name = $2, category = $3, description = $4, price_cents = $5, active = $6, updated_at = now()
		 WHERE uid = $1`,
		p.UID(), p.Name(), string(p.Category()), p.Description(), p.PriceCents(), p.Active())
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errProductNotFound
	}
	return nil
}

// AdjustStock é atômico: o UPDATE condicional trava a linha, então duas baixas simultâneas
// são serializadas e a segunda reavalia o saldo já atualizado. Nunca vende o que não existe.
func (r *Repository) AdjustStock(ctx context.Context, m ports.StockMovement) (int, error) {
	if _, err := uuid.Parse(m.ProductUID); err != nil {
		return 0, errProductNotFound
	}
	var newStock int
	err := r.inTx(ctx, func(tx *sqlx.Tx) error {
		err := tx.GetContext(ctx, &newStock, `
			UPDATE products SET stock = stock + $2, updated_at = now()
			 WHERE uid = $1 AND stock + $2 >= 0
			RETURNING stock`, m.ProductUID, m.Delta)
		if errors.Is(err, sql.ErrNoRows) {
			var exists bool
			if err := tx.GetContext(ctx, &exists, `SELECT EXISTS (SELECT 1 FROM products WHERE uid = $1)`, m.ProductUID); err != nil {
				return err
			}
			if !exists {
				return errProductNotFound
			}
			return httperr.ConflictError("estoque insuficiente")
		}
		if err != nil {
			return err
		}
		return insertMovement(ctx, tx, m)
	})
	return newStock, err
}

func insertMovement(ctx context.Context, tx *sqlx.Tx, m ports.StockMovement) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO stock_movements (product_uid, delta, reason, admin_uid)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid)`, m.ProductUID, m.Delta, m.Reason, m.AdminUID)
	return err
}

func (r *Repository) inTx(ctx context.Context, fn func(*sqlx.Tx) error) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

var errProductNotFound = httperr.NotFoundError("produto não encontrado")

func (r *Repository) GetBySlug(ctx context.Context, slug string) (catalog.Product, error) {
	var rw row
	err := r.db.GetContext(ctx, &rw, `SELECT `+columns+` FROM products WHERE slug = $1 AND active`, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errProductNotFound
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
	var where []string
	if !f.IncludeInactive {
		where = append(where, "active")
	}
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
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	query := fmt.Sprintf(`SELECT %s FROM products %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		columns, clause, order, len(args)-1, len(args))

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
