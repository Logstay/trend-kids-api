-- Configurações públicas da loja (contato e identidade), editadas pelo painel.
-- Linha única: o CHECK (id = 1) garante que nunca existe mais de uma.
CREATE TABLE IF NOT EXISTS store_settings (
    id          SMALLINT    PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    name        TEXT        NOT NULL,
    city        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    whatsapp    TEXT        NOT NULL DEFAULT '', -- dígitos nacionais (DDD + número); vazio = não configurado
    phone       TEXT        NOT NULL DEFAULT '',
    instagram   TEXT        NOT NULL DEFAULT '', -- só o usuário, sem @
    updated_by  UUID,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO store_settings (id, name, city, description)
VALUES (1, 'Trend Kids JP', 'João Pessoa · PB',
        'Crocs infantis em João Pessoa: Classic, com luz, sandálias e charms. Numeração do C4 ao J3 e entrega no mesmo dia em JP.')
ON CONFLICT (id) DO NOTHING;
