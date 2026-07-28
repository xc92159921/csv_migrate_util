DO $$
DECLARE
    target_tbl  TEXT := 'promocodes';
    columns_lst TEXT := 'promocode,discount,discount_type';
    csv_path    TEXT := '/data/7.promocodes.csv';
BEGIN
    -- 1. Создаём временную таблицу, все колонки TEXT
    EXECUTE 'CREATE TEMP TABLE temp_csv_import ("promocode" TEXT, "discount" TEXT, "discount_type" TEXT) ON COMMIT DROP';

    -- 2. Импортируем CSV
    BEGIN
        EXECUTE 'COPY temp_csv_import ("promocode", "discount", "discount_type") FROM ' || quote_literal(csv_path) || ' DELIMITER '','' CSV HEADER';
    EXCEPTION
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл % не найден, пропускаем импорт.', csv_path;
            RETURN;
    END;

    -- 3. UPSERT по id
    EXECUTE 'INSERT INTO ' || quote_ident(target_tbl) || ' ("promocode", "discount", "discount_type")
            SELECT "promocode", "discount", "discount_type" FROM temp_csv_import
            ON CONFLICT (id) DO UPDATE SET "promocode" = EXCLUDED."promocode", "discount" = EXCLUDED."discount", "discount_type" = EXCLUDED."discount_type"';

    RAISE NOTICE 'Импорт в таблицу % успешно выполнен (UPSERT по id).', target_tbl;
END $$;
