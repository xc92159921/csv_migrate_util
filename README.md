# csv_migrate_util

Утилита на Go для генерации SQL-миграций из CSV-файлов.

## Установка

```bash
go install github.com/xc92159921/csv_migrate_util@latest
```

После установки бинарь `csv_migrate_util` появится в `$GOBIN` (по умолчанию `~/go/bin`).

## Использование

1. Положите CSV-файлы в отдельную папку (например, `./csv_source`).
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
   csv_migrate_util
   ```

   Утилита:
   - удалит из папки `sql` все ранее сгенерированные `*_CSV.sql`;
   - для каждого `*.csv` из папки `csv` создаст файл `<TIMESTAMP>_<NAME_UPPER>_CSV.sql` в папке `sql` с шаблоном:

   ```sql
   DO $$
   BEGIN
       BEGIN
           COPY <table> (<columns>)
           FROM '<path>'
           DELIMITER ',' CSV HEADER;
       EXCEPTION
           WHEN undefined_file THEN
               RAISE NOTICE 'Файл <filename> не найден, пропускаем импорт данных.';
       END;
   END $$;
   ```

## Пример

`csv_source/blogs.csv`:

```
id,title,description
1,Hello,World
```

`sql_target/20260101120000_BLOGS_CSV.sql`:

```sql
DO $$
BEGIN
    BEGIN
        COPY blogs (id,title,description)
        FROM '/data/blogs.csv'
        DELIMITER ',' CSV HEADER;
    EXCEPTION
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл blogs.csv не найден, пропускаем импорт данных.';
    END;
END $$;
```

Подробности и эталонный пример см. в [agents.md](./agents.md).
