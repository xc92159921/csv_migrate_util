-- Сгенерировано csv_migrate_util --copy для таблицы users (init_only: ON CONFLICT (id) DO NOTHING)
INSERT INTO users (email,name) VALUES
    ('bob@example.com', 'Bob'),
    ('carol@example.com', 'Carol')
ON CONFLICT (id) DO NOTHING;

-- Сгенерировано csv_migrate_util --copy для таблицы users
INSERT INTO users (email,name) VALUES
    ('alice@example.com', 'Alice')
ON CONFLICT (id) DO UPDATE SET
    email = EXCLUDED.email,
    name = EXCLUDED.name;
