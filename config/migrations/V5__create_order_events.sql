-- Histórico de status: quem mudou, de quê, para quê e quando. from_status NULL = criação do pedido.
CREATE TABLE IF NOT EXISTS order_events (
    id          BIGSERIAL PRIMARY KEY,
    order_uid   UUID        NOT NULL REFERENCES orders (uid),
    from_status TEXT,
    to_status   TEXT        NOT NULL CHECK (to_status IN ('new', 'paid', 'shipped', 'delivered', 'cancelled')),
    admin_uid   UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_order_events_order ON order_events (order_uid, id);
