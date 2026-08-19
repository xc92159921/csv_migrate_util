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
   csv_migrate_util
   ```

   Утилита работает только в одном режиме — `--copy`: данные CSV
   встраиваются прямо в SQL в виде `INSERT ... VALUES (...), (...) ...
   ON CONFLICT (id) DO UPDATE SET ...`. Никаких флагов режима не требуется.

   Требование к CSV: в заголовке должна быть колонка `id` (она используется
   как ключ UPSERT). Если её нет — утилита вернёт ошибку.

   Имя sql-файла: `<YYYYMMDDHHMMSS><N>_<NAME_UPPER>_CSV.sql`.

   Утилита:
   - удалит из папки `sql` все ранее сгенерированные `*_CSV.sql`;
   - для каждого `<N>.<TABLE_NAME>.csv` из папки `csv` создаст файл
     `<YYYYMMDDHHMMSS><N>_<NAME_UPPER>_CSV.sql` в папке `sql`
     (где `<N>` берётся ровно из имени входного CSV). Содержимое — `INSERT
     ... VALUES ... ON CONFLICT (id) DO UPDATE SET ...`.

## Пример

`csv_source/10.users.csv`:

```
id,email,name
1,alice@example.com,Alice
2,bob@example.com,Bob
```

`sql_target/<ts>10_USERS_CSV.sql`:

```sql
-- Сгенерировано csv_migrate_util --copy для таблицы users
INSERT INTO users (id,email,name) VALUES
    ('1', 'alice@example.com', 'Alice'),
    ('2', 'bob@example.com', 'Bob'),
    ('3', 'carol@example.com', 'Carol')
ON CONFLICT (id) DO UPDATE SET
    email = EXCLUDED.email,
    name  = EXCLUDED.name;
```
