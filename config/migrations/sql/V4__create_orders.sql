CREATE TABLE IF NOT EXISTS orders (
    id             BIGSERIAL PRIMARY KEY,
    uid            UUID        NOT NULL UNIQUE,
    code           TEXT        NOT NULL UNIQUE,
    customer_name  TEXT        NOT NULL,
    customer_phone TEXT        NOT NULL,
    neighborhood   TEXT        NOT NULL,
    status         TEXT        NOT NULL DEFAULT 'new'
                   CHECK (status IN ('new', 'paid', 'shipped', 'delivered', 'cancelled')),
    subtotal_cents BIGINT      NOT NULL CHECK (subtotal_cents >= 0),
    shipping_cents BIGINT      NOT NULL CHECK (shipping_cents >= 0),
    total_cents    BIGINT      NOT NULL,
    payment_method TEXT        NOT NULL DEFAULT 'whatsapp',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (total_cents = subtotal_cents + shipping_cents)
);

CREATE INDEX IF NOT EXISTS idx_orders_status_created ON orders (status, created_at DESC);

-- Itens com SNAPSHOT de nome e preço: o catálogo muda, o pedido não.
CREATE TABLE IF NOT EXISTS order_items (
    id               BIGSERIAL PRIMARY KEY,
    order_uid        UUID    NOT NULL REFERENCES orders (uid),
    product_uid      UUID    NOT NULL REFERENCES products (uid),
    name_snapshot    TEXT    NOT NULL,
    unit_price_cents BIGINT  NOT NULL CHECK (unit_price_cents > 0),
    quantity         INTEGER NOT NULL CHECK (quantity > 0)
);

CREATE INDEX IF NOT EXISTS idx_order_items_order ON order_items (order_uid);

-- Quem moveu o estoque por causa de um pedido.
ALTER TABLE stock_movements ADD COLUMN IF NOT EXISTS order_uid UUID REFERENCES orders (uid);
