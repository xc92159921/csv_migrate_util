DO $$
DECLARE
    -- === ЭТИ ТРИ ПЕРЕМЕННЫЕ ПОДСТАВЛЯЕТ ГЕНЕРАТОР ===
    target_tbl  TEXT := 'blogs';                       -- Имя таблицы
    columns_lst TEXT := 'id,title,description,preview,preview_small,show_on_main,url,article,views,user_blogs';                     -- Колонки из CSV через запятую
    csv_path    TEXT := '/data/1.blogs.csv';                        -- Путь к CSV-файлу
    -- =================================================

    temp_tbl_fields TEXT;
    conflict_cols   TEXT;
    update_set      TEXT;
    final_sql       TEXT;
BEGIN
    -- 1. Превращаем список 'col1,col2' в определение для таблицы: 'col1 TEXT, col2 TEXT'
    SELECT string_agg(format('%I TEXT', trim(col)), ', ')
    INTO temp_tbl_fields
    FROM unnest(string_to_array(columns_lst, ',')) AS col;

    -- 2. Создаем временную таблицу, где все типы TEXT
    EXECUTE format('CREATE TEMP TABLE temp_csv_import (%s) ON COMMIT DROP', temp_tbl_fields);

    -- 3. Безопасно импортируем CSV-данные во временную таблицу
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

    -- 4. Ищем уникальный ключ таблицы (PRIMARY KEY в приоритете, иначе UNIQUE)
    SELECT string_agg(format('%I', att.attname), ', ')
    INTO conflict_cols
    FROM pg_index i
    JOIN pg_attribute att ON att.attrelid = i.indrelid AND att.attnum = ANY(i.indkey)
    WHERE i.indrelid = target_tbl::regclass 
      AND i.indisunique
    GROUP BY i.indexrelid, i.indisprimary
    ORDER BY i.indisprimary DESC
    LIMIT 1;

    -- 5. Если уникальный ключ найден — строим UPSERT
    IF conflict_cols IS NOT NULL AND conflict_cols != '' THEN
        SELECT string_agg(format('%1$I = EXCLUDED.%1$I', trim(col)), ', ')
        INTO update_set
        FROM unnest(string_to_array(columns_lst, ',')) AS col
        WHERE trim(col) NOT IN (
            SELECT trim(c) FROM unnest(string_to_array(conflict_cols, ',')) c
        );

        IF update_set IS NULL OR update_set = '' THEN
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
    ELSE
        -- 6. Если у таблицы вообще нет уникальных ключей — просто дописываем
        final_sql := format('
            INSERT INTO %1$I (%2$s)
            SELECT %2$s FROM temp_csv_import',
            target_tbl, columns_lst
        );
    END IF;

    -- 7. Выполняем один итоговый запрос
    EXECUTE final_sql;

    RAISE NOTICE 'Импорт в таблицу % успешно выполнен (UPSERT).', target_tbl;
END $$;
