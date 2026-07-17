# csv_migrate_util

Утилита на Go для генерации SQL-миграций из CSV-файлов. Использует
[spf13/cobra](https://github.com/spf13/cobra) для CLI.

## Установка

```bash
go install github.com/xc92159921/csv_migrate_util@latest
```

После установки бинарь `csv_migrate_util` появится в `$GOBIN` (по умолчанию `~/go/bin`).

## Использование

1. Положите CSV-файлы в отдельную папку (например, `./csv_source`).
   Имя каждого файла должно быть в формате `<N>.<TABLE_NAME>.csv`,
   где `<N>` — положительное целое число без ведущих нулей
   (например, `1.blogs.csv`, `2.users.csv`, `10.posts.csv`).
   Имена без числового префикса или с дублирующимся `<N>` — ошибка.
2. Создайте `csv_migrate_config.json` в корне проекта:

   ```json
   {
     "csv": "./csv_source",
     "sql": "./sql_target",
     "target": "/data"
   }
   ```

   - `csv` — папка с исходными `.csv` (обязательно).
   - `sql` — папка для сгенерированных `.sql` (обязательно).
   - `target` — префикс пути в `COPY ... FROM` (например, `/data` для Docker-монтирования). Можно оставить пустым.

   Если файла нет — утилита создаст его с дефолтными пустыми значениями при первом запуске.

3. Запустите утилиту из корня проекта:

   ```bash
   # обычный режим — прямая COPY в целевую таблицу
   csv_migrate_util

   # режим temp_table — импорт через временную таблицу с UPSERT по PK/UNIQUE
   csv_migrate_util --temp-table
   # или короткий алиас
   csv_migrate_util -t
   ```

   Имя sql-файла в обоих режимах одинаковое:
   `<YYYYMMDDHHMMSS><N>_<NAME_UPPER>_CSV.sql`. Отличается только
   **содержимое** файла.

   Утилита:
   - удалит из папки `sql` все ранее сгенерированные `*_CSV.sql`;
   - для каждого `<N>.<TABLE_NAME>.csv` из папки `csv` создаст файл
     `<YYYYMMDDHHMMSS><N>_<NAME_UPPER>_CSV.sql` в папке `sql`
     (где `<N>` берётся ровно из имени входного CSV). В обычном режиме
     содержимое — шаблон с прямым `COPY`, в режиме `temp_table` — шаблон
     с импортом во временную таблицу и UPSERT.

## Пример (обычный режим)

`csv_source/1.blogs.csv`:

```
id,title,description
1,Hello,World
```

`sql_target/202601011200001_BLOGS_CSV.sql`:

```sql
DO $$
BEGIN
    BEGIN
        COPY blogs (id,title,description)
        FROM '/data/1.blogs.csv'
        DELIMITER ',' CSV HEADER;
    EXCEPTION
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл 1.blogs.csv не найден, пропускаем импорт данных.';
    END;
END $$;
```

## Пример (режим `temp_table`)

`csv_source/7.promocodes.csv`:

```
promocode,discount,discount_type
SUMMER10,10,percent
WINTER20,20,percent
```

`sql_target/202601011200007_PROMOCODES_CSV.sql`:

```sql
DO $$
DECLARE
    target_tbl  TEXT := 'promocodes';
    columns_lst TEXT := 'promocode,discount,discount_type';
    csv_path    TEXT := '/data/7.promocodes.csv';
    temp_tbl_fields TEXT;
    conflict_cols   TEXT;
    update_set      TEXT;
    final_sql       TEXT;
BEGIN
    SELECT string_agg(format('%I TEXT', trim(col)), ', ')
    INTO temp_tbl_fields
    FROM unnest(string_to_array(columns_lst, ',')) AS col;

    EXECUTE format('CREATE TEMP TABLE temp_csv_import (%s) ON COMMIT DROP', temp_tbl_fields);

    BEGIN
        EXECUTE format('COPY temp_csv_import (%s) FROM %L DELIMITER '','' CSV HEADER',
            columns_lst, csv_path);
    EXCEPTION
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл % не найден, пропускаем импорт.', csv_path;
            RETURN;
    END;

    SELECT string_agg(format('%I', att.attname), ', ')
    INTO conflict_cols
    FROM pg_index i
    JOIN pg_attribute att ON att.attrelid = i.indrelid AND att.attnum = ANY(i.indkey)
    WHERE i.indrelid = target_tbl::regclass
      AND i.indisunique
    GROUP BY i.indexrelid, i.indisprimary
    ORDER BY i.indisprimary DESC
    LIMIT 1;

    IF conflict_cols IS NOT NULL AND conflict_cols != '' THEN
        SELECT string_agg(format('%1$I = EXCLUDED.%1$I', trim(col)), ', ')
        INTO update_set
        FROM unnest(string_to_array(columns_lst, ',')) AS col
        WHERE trim(col) NOT IN (
            SELECT trim(c) FROM unnest(string_to_array(conflict_cols, ',')) c
        );

        IF update_set IS NULL OR update_set = '' THEN
            final_sql := format('INSERT INTO %1$I (%2$s) SELECT %2$s FROM temp_csv_import ON CONFLICT (%3$s) DO NOTHING',
                target_tbl, columns_lst, conflict_cols);
        ELSE
            final_sql := format('INSERT INTO %1$I (%2$s) SELECT %2$s FROM temp_csv_import ON CONFLICT (%3$s) DO UPDATE SET %4$s',
                target_tbl, columns_lst, conflict_cols, update_set);
        END IF;
    ELSE
        final_sql := format('INSERT INTO %1$I (%2$s) SELECT %2$s FROM temp_csv_import',
            target_tbl, columns_lst);
    END IF;

    EXECUTE final_sql;
    RAISE NOTICE 'Импорт в таблицу % успешно выполнен (UPSERT).', target_tbl;
END $$;
```

Подробности и эталонный пример см. в [agents.md](./agents.md).
