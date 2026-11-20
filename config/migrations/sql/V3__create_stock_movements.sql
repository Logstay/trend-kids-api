-- Livro-razão do estoque: cada entrada/saída fica registrada, com motivo e quem fez.
CREATE TABLE IF NOT EXISTS stock_movements (
    id          BIGSERIAL PRIMARY KEY,
    product_uid UUID        NOT NULL REFERENCES products (uid),
    delta       INTEGER     NOT NULL CHECK (delta <> 0),
    reason      TEXT        NOT NULL,
    admin_uid   UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_stock_movements_product ON stock_movements (product_uid, created_at DESC);
