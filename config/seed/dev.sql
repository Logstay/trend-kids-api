-- Dados de exemplo para desenvolvimento (preços fictícios).
INSERT INTO products (uid, name, slug, category, description, price_cents, stock) VALUES
 (gen_random_uuid(), 'Classic Clog Infantil Rosa', 'classic-clog-infantil-rosa', 'Classic', 'Exemplo', 18990, 6),
 (gen_random_uuid(), 'Crocs com Luz Fun Lab', 'crocs-com-luz-fun-lab', 'Com luz', 'Exemplo', 21990, 3),
 (gen_random_uuid(), 'Sandália Crocband Infantil', 'sandalia-crocband-infantil', 'Sandálias', 'Exemplo', 15990, 4),
 (gen_random_uuid(), 'Kit 5 Charms Dinossauros', 'kit-5-charms-dinossauros', 'Charms', 'Exemplo', 4990, 20)
ON CONFLICT (slug) DO NOTHING;
