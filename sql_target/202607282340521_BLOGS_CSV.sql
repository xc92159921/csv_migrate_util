DO $$
DECLARE
    target_tbl  TEXT := 'blogs';
    columns_lst TEXT := 'id,title,description,preview,preview_small,show_on_main,url,article,views,user_blogs';
    csv_path    TEXT := '/data/1.blogs.csv';
BEGIN
    -- 1. Создаём временную таблицу, все колонки TEXT
    EXECUTE 'CREATE TEMP TABLE temp_csv_import ("id" TEXT, "title" TEXT, "description" TEXT, "preview" TEXT, "preview_small" TEXT, "show_on_main" TEXT, "url" TEXT, "article" TEXT, "views" TEXT, "user_blogs" TEXT) ON COMMIT DROP';

    -- 2. Импортируем CSV
    BEGIN
        EXECUTE 'COPY temp_csv_import ("id", "title", "description", "preview", "preview_small", "show_on_main", "url", "article", "views", "user_blogs") FROM ' || quote_literal(csv_path) || ' DELIMITER '','' CSV HEADER';
    EXCEPTION
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл % не найден, пропускаем импорт.', csv_path;
            RETURN;
    END;

    -- 3. UPSERT по id
    EXECUTE 'INSERT INTO ' || quote_ident(target_tbl) || ' ("id", "title", "description", "preview", "preview_small", "show_on_main", "url", "article", "views", "user_blogs")
            SELECT "id", "title", "description", "preview", "preview_small", "show_on_main", "url", "article", "views", "user_blogs" FROM temp_csv_import
            ON CONFLICT (id) DO UPDATE SET "title" = EXCLUDED."title", "description" = EXCLUDED."description", "preview" = EXCLUDED."preview", "preview_small" = EXCLUDED."preview_small", "show_on_main" = EXCLUDED."show_on_main", "url" = EXCLUDED."url", "article" = EXCLUDED."article", "views" = EXCLUDED."views", "user_blogs" = EXCLUDED."user_blogs"';

    RAISE NOTICE 'Импорт в таблицу % успешно выполнен (UPSERT по id).', target_tbl;
END $$;
