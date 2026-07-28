DO $$
DECLARE
    target_tbl  TEXT := 'users';
    columns_lst TEXT := 'id,email,name';
    csv_path    TEXT := '/data/10.users.csv';
BEGIN
    -- 1. Создаём временную таблицу, все колонки TEXT
    EXECUTE 'CREATE TEMP TABLE temp_csv_import ("id" TEXT, "email" TEXT, "name" TEXT) ON COMMIT DROP';

    -- 2. Импортируем CSV
    BEGIN
        EXECUTE 'COPY temp_csv_import ("id", "email", "name") FROM ' || quote_literal(csv_path) || ' DELIMITER '','' CSV HEADER';
    EXCEPTION
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл % не найден, пропускаем импорт.', csv_path;
            RETURN;
    END;

    -- 3. UPSERT по id
    EXECUTE 'INSERT INTO ' || quote_ident(target_tbl) || ' ("id", "email", "name")
            SELECT "id", "email", "name" FROM temp_csv_import
            ON CONFLICT (id) DO UPDATE SET "email" = EXCLUDED."email", "name" = EXCLUDED."name"';

    RAISE NOTICE 'Импорт в таблицу % успешно выполнен (UPSERT по id).', target_tbl;
END $$;
