-- Várias fotos por produto, em ordem (a de posição 0 é a capa). Guarda só a CHAVE do arquivo.
CREATE TABLE IF NOT EXISTS product_images (
    id          BIGSERIAL PRIMARY KEY,
    product_uid UUID        NOT NULL REFERENCES products (uid) ON DELETE CASCADE,
    image_key   TEXT        NOT NULL UNIQUE,
    position    INTEGER     NOT NULL CHECK (position >= 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- adiado até o fim da transação: permite trocar posições sem violar a unicidade no meio do caminho
    CONSTRAINT product_images_position_key UNIQUE (product_uid, position) DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX IF NOT EXISTS idx_product_images_product ON product_images (product_uid, position);

-- A foto única da V8 vira a primeira da galeria (se a coluna ainda existir), e a coluna antiga sai.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'products' AND column_name = 'image_key') THEN
        INSERT INTO product_images (product_uid, image_key, position)
        SELECT uid, image_key, 0 FROM products WHERE image_key <> '';
        -- (sem ON CONFLICT: ele não funciona com a restrição de posição adiável, e não é necessário: cada produto
        -- entra uma vez na posição 0, e rodar de novo é seguro porque a coluna some no fim deste bloco)
        ALTER TABLE products DROP COLUMN image_key;
    END IF;
END $$;
