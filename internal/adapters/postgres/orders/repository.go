package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO order_events (order_uid, from_status, to_status) VALUES ($1, NULL, $2)`, o.UID(), string(o.Status())); err != nil {
			return err
		}

		// O estoque NÃO é baixado aqui: por enquanto o vendedor dá a baixa depois (ver README).
		for _, it := range o.Items() { // na ordem do carrinho, para a exibição bater com o que a pessoa montou
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO order_items (order_uid, product_uid, name_snapshot, unit_price_cents, quantity)
				VALUES ($1, $2, $3, $4, $5)`, o.UID(), it.ProductUID, it.Name, it.UnitPriceCents, it.Quantity); err != nil {
				return err
			}
		}
		return nil
	})
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

func (row orderRow) order(items []orders.Item) orders.Order {
	return orders.Restore(orders.Snapshot{
		UID: row.UID, Code: row.Code, CustomerName: row.CustomerName, Phone: row.CustomerPhone,
		Neighborhood: row.Neighborhood, Status: orders.Status(row.Status), Items: items,
		SubtotalCents: row.SubtotalCents, ShippingCents: row.ShippingCents, TotalCents: row.TotalCents,
		CreatedAt: row.CreatedAt,
	})
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

	itemsByOrder, err := r.itemsFor(ctx, []string{row.UID})
	if err != nil {
		return nil, err
	}
	items := itemsByOrder[row.UID]

	return row.order(items), nil
}

type orderItemRow struct {
	OrderUID string `db:"order_uid"`
	itemRow
}

// itemsFor carrega os itens de vários pedidos de uma vez (sem N+1), na ordem do carrinho.
func (r *Repository) itemsFor(ctx context.Context, orderUIDs []string) (map[string][]orders.Item, error) {
	out := make(map[string][]orders.Item, len(orderUIDs))
	if len(orderUIDs) == 0 {
		return out, nil
	}
	var rows []orderItemRow
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT order_uid::text AS order_uid, product_uid::text AS product_uid, name_snapshot, unit_price_cents, quantity
		  FROM order_items WHERE order_uid = ANY($1::uuid[]) ORDER BY id`, pq.Array(orderUIDs)); err != nil {
		return nil, err
	}
	for _, it := range rows {
		out[it.OrderUID] = append(out[it.OrderUID], orders.Item{
			ProductUID: it.ProductUID, Name: it.Name, UnitPriceCents: it.UnitPriceCents, Quantity: it.Quantity,
		})
	}
	return out, nil
}

func (r *Repository) List(ctx context.Context, f ports.ListFilter) ([]orders.Order, error) {
	var rows []orderRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT uid, code, customer_name, customer_phone, neighborhood, status,
		       subtotal_cents, shipping_cents, total_cents, created_at
		  FROM orders
		 WHERE ($1 = '' OR status = $1)
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2 OFFSET $3`, f.Status, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}

	uids := make([]string, 0, len(rows))
	for _, row := range rows {
		uids = append(uids, row.UID)
	}
	items, err := r.itemsFor(ctx, uids)
	if err != nil {
		return nil, err
	}

	result := make([]orders.Order, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.order(items[row.UID]))
	}
	return result, nil
}

func (r *Repository) Events(ctx context.Context, orderUID string) ([]ports.StatusEvent, error) {
	var rows []struct {
		From  sql.NullString `db:"from_status"`
		To    string         `db:"to_status"`
		Admin sql.NullString `db:"admin_uid"`
		At    time.Time      `db:"created_at"`
	}
	if err := r.db.SelectContext(ctx, &rows, `
		SELECT from_status, to_status, admin_uid::text AS admin_uid, created_at
		  FROM order_events WHERE order_uid = $1 ORDER BY id`, orderUID); err != nil {
		return nil, err
	}
	events := make([]ports.StatusEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, ports.StatusEvent{
			From: orders.Status(row.From.String), To: orders.Status(row.To), AdminUID: row.Admin.String, At: row.At,
		})
	}
	return events, nil
}

// UpdateStatus trava a linha do pedido (FOR UPDATE), então duas mudanças simultâneas são serializadas
// e a segunda enxerga o status já atualizado. Por isso um duplo clique em "cancelar" devolve o estoque uma vez só.
func (r *Repository) UpdateStatus(ctx context.Context, c ports.ChangeStatus) (orders.Order, error) {
	err := r.inTx(ctx, func(tx *sqlx.Tx) error {
		var cur struct {
			UID    string `db:"uid"`
			Code   string `db:"code"`
			Status string `db:"status"`
		}
		err := tx.GetContext(ctx, &cur, `SELECT uid, code, status FROM orders WHERE code = $1 FOR UPDATE`, c.Code)
		if errors.Is(err, sql.ErrNoRows) {
			return httperr.NotFoundError("pedido não encontrado")
		}
		if err != nil {
			return err
		}

		from := orders.Status(cur.Status)
		if !orders.CanTransition(from, c.To) {
			return httperr.ConflictError(fmt.Sprintf("transição de status não permitida: %s → %s", from, c.To))
		}

		if _, err := tx.ExecContext(ctx, `UPDATE orders SET status = $2, updated_at = now() WHERE uid = $1`, cur.UID, string(c.To)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO order_events (order_uid, from_status, to_status, admin_uid)
			VALUES ($1, $2, $3, NULLIF($4, '')::uuid)`, cur.UID, string(from), string(c.To), c.AdminUID); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.GetByCode(ctx, c.Code)
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
