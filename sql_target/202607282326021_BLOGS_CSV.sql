DO $$
DECLARE
    -- === ЭТИ ТРИ ПЕРЕМЕННЫЕ ПОДСТАВЛЯЕТ ГЕНЕРАТОР ===
    target_tbl  TEXT := 'blogs';                       -- Имя таблицы
    columns_lst TEXT := 'id,title,description,preview,preview_small,show_on_main,url,article,views,user_blogs';                     -- Колонки из CSV через запятую
    csv_path    TEXT := '/data/1.blogs.csv';                        -- Путь к CSV-файлу
    -- =================================================

    conflict_cols   CONSTANT TEXT := 'id';
    update_set      TEXT;
    final_sql       TEXT;
BEGIN
    -- 1. Создаем временную таблицу, где все типы TEXT
    EXECUTE format('CREATE TEMP TABLE temp_csv_import (%s) ON COMMIT DROP',
        (SELECT string_agg(format('%I TEXT', trim(col)), ', ')
         FROM unnest(string_to_array(columns_lst, ',')) AS col));

    -- 2. Безопасно импортируем CSV-данные во временную таблицу
    BEGIN
        EXECUTE format('
            COPY temp_csv_import (%s)
            FROM %L
            DELIMITER '','' CSV HEADER',
            columns_lst, csv_path
        );
    EXCEPTION
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл % не найден, пропускаем импорт.', csv_path;
            RETURN;
    END;

    -- 3. Строим SET-часть: обновляем все колонки кроме id
    SELECT string_agg(format('%1$I = EXCLUDED.%1$I', trim(col)), ', ')
    INTO update_set
    FROM unnest(string_to_array(columns_lst, ',')) AS col
    WHERE trim(col) <> conflict_cols;

    IF update_set IS NULL OR update_set = '' THEN
        -- Если в CSV только колонка id — ничего обновлять нечего, просто пропускаем дубли
        final_sql := format('
            INSERT INTO %1$I (%2$s)
            SELECT %2$s FROM temp_csv_import
            ON CONFLICT (%3$s) DO NOTHING',
            target_tbl, columns_lst, conflict_cols
        );
    ELSE
        final_sql := format('
            INSERT INTO %1$I (%2$s)
            SELECT %2$s FROM temp_csv_import
            ON CONFLICT (%3$s)
            DO UPDATE SET %4$s',
            target_tbl, columns_lst, conflict_cols, update_set
        );
    END IF;

    EXECUTE final_sql;

    RAISE NOTICE 'Импорт в таблицу % успешно выполнен (UPSERT по id).', target_tbl;
END $$;
