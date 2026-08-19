-- Сгенерировано csv_migrate_util --copy для таблицы posts
INSERT INTO posts (id,title,body,author_id) VALUES
    ('1', 'Hello', 'World', '1'),
    ('2', 'Second post', 'Lorem ipsum', '2')
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    body = EXCLUDED.body,
    author_id = EXCLUDED.author_id;
