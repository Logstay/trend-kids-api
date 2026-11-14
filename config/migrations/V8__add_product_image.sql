-- Foto principal do produto: guarda só a CHAVE do arquivo (gerada pelo servidor); a URL é montada na hora.
ALTER TABLE products ADD COLUMN IF NOT EXISTS image_key TEXT NOT NULL DEFAULT '';
