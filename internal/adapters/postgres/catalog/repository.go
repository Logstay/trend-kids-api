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
	"github.com/lib/pq"
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

func (r row) product(images []string) catalog.Product {
	return catalog.Restore(catalog.Snapshot{
		UID: r.UID, Name: r.Name, Slug: r.Slug, Category: r.Category, Description: r.Description,
		PriceCents: r.PriceCents, Stock: r.Stock, Active: r.Active, Images: images, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	})
}

// imagesFor carrega as fotos de vários produtos de uma vez (sem N+1), em ordem de exibição.
func (r *Repository) imagesFor(ctx context.Context, uids []string) (map[string][]string, error) {
	out := make(map[string][]string, len(uids))
	if len(uids) == 0 {
		return out, nil
	}
	var rows []struct {
		ProductUID string `db:"product_uid"`
		Key        string `db:"image_key"`
	}
	if err := r.db.SelectContext(ctx, &rows,
		`SELECT product_uid::text AS product_uid, image_key FROM product_images WHERE product_uid = ANY($1::uuid[]) ORDER BY product_uid, position`,
		pq.Array(uids)); err != nil {
		return nil, err
	}
	for _, rw := range rows {
		out[rw.ProductUID] = append(out[rw.ProductUID], rw.Key)
	}
	return out, nil
}

// one monta um produto só (com as fotos).
func (r *Repository) one(ctx context.Context, rw row) (catalog.Product, error) {
	imgs, err := r.imagesFor(ctx, []string{rw.UID})
	if err != nil {
		return nil, err
	}
	return rw.product(imgs[rw.UID]), nil
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
	return r.one(ctx, rw)
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

// lockProduct trava a linha do produto: adições, remoções e reordenações da galeria do MESMO produto
// passam a andar em fila, o que garante o limite de fotos e a ordem mesmo com várias pessoas ao mesmo tempo.
func lockProduct(ctx context.Context, tx *sqlx.Tx, uid string) error {
	var locked string
	err := tx.GetContext(ctx, &locked, `SELECT uid::text FROM products WHERE uid = $1 FOR UPDATE`, uid)
	if errors.Is(err, sql.ErrNoRows) {
		return errProductNotFound
	}
	return err
}

func (r *Repository) AddImage(ctx context.Context, uid, key string) error {
	if _, err := uuid.Parse(uid); err != nil {
		return errProductNotFound
	}
	return r.inTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockProduct(ctx, tx, uid); err != nil {
			return err
		}
		var count, next int
		if err := tx.QueryRowxContext(ctx,
			`SELECT count(*), coalesce(max(position) + 1, 0) FROM product_images WHERE product_uid = $1`, uid).Scan(&count, &next); err != nil {
			return err
		}
		if count >= ports.MaxImages {
			return httperr.ConflictError(fmt.Sprintf("limite de %d fotos por produto atingido", ports.MaxImages))
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO product_images (product_uid, image_key, position) VALUES ($1, $2, $3)`, uid, key, next)
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			switch pqErr.Constraint {
			case "product_images_image_key_key":
				return httperr.ConflictError("essa foto já está cadastrada")
			case "product_images_position_key": // rede de segurança: duas adições tentaram a mesma posição
				return httperr.ConflictError("outra foto foi adicionada ao mesmo tempo; tente de novo")
			}
		}
		return err
	})
}

func (r *Repository) RemoveImage(ctx context.Context, uid, key string) error {
	if _, err := uuid.Parse(uid); err != nil {
		return errProductNotFound
	}
	return r.inTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockProduct(ctx, tx, uid); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM product_images WHERE product_uid = $1 AND image_key = $2`, uid, key)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return httperr.NotFoundError("foto não encontrada")
		}
		// Fecha o buraco: as posições voltam a ser 0, 1, 2... (a restrição de unicidade só é conferida no fim).
		_, err = tx.ExecContext(ctx, `
			UPDATE product_images p SET position = s.rn - 1
			  FROM (SELECT id, row_number() OVER (ORDER BY position) AS rn FROM product_images WHERE product_uid = $1) s
			 WHERE p.id = s.id`, uid)
		return err
	})
}

func (r *Repository) ReorderImages(ctx context.Context, uid string, keys []string) error {
	if _, err := uuid.Parse(uid); err != nil {
		return errProductNotFound
	}
	return r.inTx(ctx, func(tx *sqlx.Tx) error {
		if err := lockProduct(ctx, tx, uid); err != nil {
			return err
		}
		var current []string
		if err := tx.SelectContext(ctx, &current, `SELECT image_key FROM product_images WHERE product_uid = $1`, uid); err != nil {
			return err
		}
		if !samePhotos(current, keys) {
			return httperr.RequiredError("a nova ordem precisa ter exatamente as fotos atuais, sem repetir")
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE product_images pi SET position = t.ord - 1
			  FROM unnest($2::text[]) WITH ORDINALITY AS t(key, ord)
			 WHERE pi.product_uid = $1 AND pi.image_key = t.key`, uid, pq.Array(keys))
		return err
	})
}

// samePhotos: as duas listas têm exatamente os mesmos itens (qualquer ordem, sem repetir).
func samePhotos(current, wanted []string) bool {
	if len(current) != len(wanted) {
		return false
	}
	seen := make(map[string]bool, len(wanted))
	for _, k := range wanted {
		if seen[k] {
			return false
		}
		seen[k] = true
	}
	for _, k := range current {
		if !seen[k] {
			return false
		}
	}
	return true
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
	return r.one(ctx, rw)
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
	uids := make([]string, 0, len(rows))
	for _, rw := range rows {
		uids = append(uids, rw.UID)
	}
	imgs, err := r.imagesFor(ctx, uids)
	if err != nil {
		return nil, err
	}
	result := make([]catalog.Product, 0, len(rows))
	for _, rw := range rows {
		result = append(result, rw.product(imgs[rw.UID]))
	}
	return result, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
