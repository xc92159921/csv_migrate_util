-- Сгенерировано csv_migrate_util --copy для таблицы users
INSERT INTO users (id,email,name) VALUES
    ('1', 'alice@example.com', 'Alice'),
    ('2', 'bob@example.com', 'Bob'),
    ('3', 'carol@example.com', 'Carol')
ON CONFLICT (id) DO UPDATE SET
    email = EXCLUDED.email,
    name = EXCLUDED.name;
