DO $$
DECLARE
    target_tbl  TEXT := 'posts';
    columns_lst TEXT := 'id,title,body,author_id';
    csv_path    TEXT := '/data/11.posts.csv';
BEGIN
    -- 1. Создаём временную таблицу, все колонки TEXT
    EXECUTE 'CREATE TEMP TABLE temp_csv_import ("id" TEXT, "title" TEXT, "body" TEXT, "author_id" TEXT) ON COMMIT DROP';

    -- 2. Импортируем CSV
    BEGIN
        EXECUTE 'COPY temp_csv_import ("id", "title", "body", "author_id") FROM ' || quote_literal(csv_path) || ' DELIMITER '','' CSV HEADER';
    EXCEPTION
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл % не найден, пропускаем импорт.', csv_path;
            RETURN;
    END;

    -- 3. UPSERT по id
    EXECUTE 'INSERT INTO ' || quote_ident(target_tbl) || ' ("id", "title", "body", "author_id")
            SELECT "id", "title", "body", "author_id" FROM temp_csv_import
            ON CONFLICT (id) DO UPDATE SET "title" = EXCLUDED."title", "body" = EXCLUDED."body", "author_id" = EXCLUDED."author_id"';

    RAISE NOTICE 'Импорт в таблицу % успешно выполнен (UPSERT по id).', target_tbl;
END $$;
