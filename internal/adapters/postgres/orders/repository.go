package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Logstay/trend-kids-api/internal/domain/orders"
	ports "github.com/Logstay/trend-kids-api/internal/ports/orders"
	httperr "github.com/Logstay/trend-kids-api/pkg/errors"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type Repository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) *Repository { return &Repository{db: db} }

// Create grava pedido, itens e baixa de estoque numa transação só: ou tudo entra, ou nada.
func (r *Repository) Create(ctx context.Context, o orders.Order) error {
	return r.inTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO orders (uid, code, customer_name, customer_phone, neighborhood, status,
			                    subtotal_cents, shipping_cents, total_cents)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			o.UID(), o.Code(), o.CustomerName(), o.CustomerPhone(), o.Neighborhood(), string(o.Status()),
			o.SubtotalCents(), o.ShippingCents(), o.TotalCents())
		if isCodeTaken(err) {
			return ports.ErrCodeTaken
		}
		if err != nil {
			return err
		}

		items := o.Items()
		for _, it := range items { // na ordem do carrinho, para a exibição bater com o que a pessoa montou
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO order_items (order_uid, product_uid, name_snapshot, unit_price_cents, quantity)
				VALUES ($1, $2, $3, $4, $5)`, o.UID(), it.ProductUID, it.Name, it.UnitPriceCents, it.Quantity); err != nil {
				return err
			}
		}

		// Baixa de estoque em ORDEM FIXA de produto. Dois pedidos com os mesmos produtos em ordem
		// oposta travariam um ao outro (deadlock) se bloqueassem na ordem do carrinho.
		sort.Slice(items, func(i, j int) bool { return items[i].ProductUID < items[j].ProductUID })
		for _, it := range items {
			if err := decrementStock(ctx, tx, o, it); err != nil {
				return err
			}
		}
		return nil
	})
}

// decrementStock é atômico: o UPDATE condicional trava a linha e reavalia o saldo já atualizado.
func decrementStock(ctx context.Context, tx *sqlx.Tx, o orders.Order, it orders.Item) error {
	var stock int
	err := tx.GetContext(ctx, &stock, `
		UPDATE products SET stock = stock - $2, updated_at = now()
		 WHERE uid = $1 AND active AND stock >= $2
		RETURNING stock`, it.ProductUID, it.Quantity)
	if errors.Is(err, sql.ErrNoRows) {
		var active bool
		if err := tx.GetContext(ctx, &active, `SELECT active FROM products WHERE uid = $1`, it.ProductUID); err == nil && !active {
			return httperr.ConflictError(fmt.Sprintf("produto indisponível: %s", it.Name))
		}
		return httperr.ConflictError(fmt.Sprintf("estoque insuficiente: %s", it.Name))
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO stock_movements (product_uid, delta, reason, order_uid)
		VALUES ($1, $2, $3, $4)`, it.ProductUID, -it.Quantity, "pedido "+o.Code(), o.UID())
	return err
}

type orderRow struct {
	UID           string    `db:"uid"`
	Code          string    `db:"code"`
	CustomerName  string    `db:"customer_name"`
	CustomerPhone string    `db:"customer_phone"`
	Neighborhood  string    `db:"neighborhood"`
	Status        string    `db:"status"`
	SubtotalCents int64     `db:"subtotal_cents"`
	ShippingCents int64     `db:"shipping_cents"`
	TotalCents    int64     `db:"total_cents"`
	CreatedAt     time.Time `db:"created_at"`
}

type itemRow struct {
	ProductUID     string `db:"product_uid"`
	Name           string `db:"name_snapshot"`
	UnitPriceCents int64  `db:"unit_price_cents"`
	Quantity       int    `db:"quantity"`
}

func (r *Repository) GetByCode(ctx context.Context, code string) (orders.Order, error) {
	var row orderRow
	err := r.db.GetContext(ctx, &row, `
		SELECT uid, code, customer_name, customer_phone, neighborhood, status,
		       subtotal_cents, shipping_cents, total_cents, created_at
		  FROM orders WHERE code = $1`, code)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, httperr.NotFoundError("pedido não encontrado")
	}
	if err != nil {
		return nil, err
	}

	var rows []itemRow
	if err := r.db.SelectContext(ctx, &rows,
		`SELECT product_uid::text, name_snapshot, unit_price_cents, quantity FROM order_items WHERE order_uid = $1 ORDER BY id`, row.UID); err != nil {
		return nil, err
	}
	items := make([]orders.Item, 0, len(rows))
	for _, it := range rows {
		items = append(items, orders.Item{ProductUID: it.ProductUID, Name: it.Name, UnitPriceCents: it.UnitPriceCents, Quantity: it.Quantity})
	}

	return orders.Restore(orders.Snapshot{
		UID: row.UID, Code: row.Code, CustomerName: row.CustomerName, Phone: row.CustomerPhone,
		Neighborhood: row.Neighborhood, Status: orders.Status(row.Status), Items: items,
		SubtotalCents: row.SubtotalCents, ShippingCents: row.ShippingCents, TotalCents: row.TotalCents,
		CreatedAt: row.CreatedAt,
	}), nil
}

func isCodeTaken(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == "orders_code_key"
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
