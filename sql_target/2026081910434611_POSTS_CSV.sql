-- Сгенерировано csv_migrate_util --copy для таблицы posts
INSERT INTO posts (title,body,author_id) VALUES
    ('Hello', 'World', '1'),
    ('Second post', 'Lorem ipsum', '2')
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    author_id = EXCLUDED.author_id;
