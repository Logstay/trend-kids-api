-- Vendedoras: para quem o cliente é levado no WhatsApp. O contato geral da loja fica em store_settings.
CREATE TABLE IF NOT EXISTS sellers (
    id         BIGSERIAL PRIMARY KEY,
    uid        UUID        NOT NULL UNIQUE,
    name       TEXT        NOT NULL,
    whatsapp   TEXT        NOT NULL UNIQUE, -- dígitos nacionais (DDD + número); único: dois cadastros não dividem o número
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sellers_active ON sellers (created_at) WHERE active;
