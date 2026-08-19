-- Сгенерировано csv_migrate_util --copy для таблицы blogs
INSERT INTO blogs (title,description,preview,preview_small,show_on_main,url,article,views,user_blogs) VALUES
    ('Тестовая статья', 'Описание статьи', '/assets/blog/preview.jpg', '/assets/blog/preview_small.jpg', 'false', 'test-article-1', '# Тестовая статья про гранит и качество натурального камня оптом дешево', '3', '11111111-1111-1111-1111-111111111111')
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    preview = EXCLUDED.preview,
    preview_small = EXCLUDED.preview_small,
    show_on_main = EXCLUDED.show_on_main,
    url = EXCLUDED.url,
    article = EXCLUDED.article,
    views = EXCLUDED.views,
    user_blogs = EXCLUDED.user_blogs;
