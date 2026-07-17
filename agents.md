# csv_migrate_util — agents.md

## Назначение

Утилита на Go для генерации SQL-миграций из CSV-файлов. Устанавливается
через `go install github.com/xc92159921/csv_migrate_util` и запускается
из корня проекта, где лежит `csv_migrate_config.json`.

Использует [spf13/cobra](https://github.com/spf13/cobra) для CLI
(один root-команда + флаги).

## Структура проекта

```
.
├── agents.md                # этот файл
├── README.md
├── go.mod                   # module app, go 1.26.4
├── go.sum
├── main.go                  # точка входа, инициализация cobra
├── cmd/                     # команды cobra (root, generate, ...)
├── internal/                # логика (config, csv, sql, render)
├── csv_migrate_config.json  # создаётся автоматически при первом запуске
├── csv_source/              # источник CSV (по умолчанию)
└── sql_target/              # куда класть сгенерированные .sql (по умолчанию)
```

(Структура `cmd/` и `internal/` — рекомендуемая для cobra-проекта,
конкретная разбивка по файлам — на усмотрение реализации.)

## CLI

Одна корневая команда `csv_migrate_util` (cobra) с флагами:

| Флаг           | Сокращение | По умолчанию | Описание                                        |
|----------------|------------|--------------|-------------------------------------------------|
| `--temp-table` | `-t`       | `false`      | Сгенерировать SQL в режиме `temp_table` (см. ниже). |

Примеры:

```bash
# обычный режим — прямая COPY в целевую таблицу
csv_migrate_util

# режим temp_table — импорт через временную таблицу + UPSERT по PK/UNIQUE
csv_migrate_util --temp-table
csv_migrate_util -t
```

## Конфигурация `csv_migrate_config.json`

Три поля:

| Поле      | Тип    | Обязательно | Описание                                      |
|-----------|--------|-------------|-----------------------------------------------|
| `csv`     | string | да          | Папка с исходными `.csv` (относительный путь) |
| `sql`     | string | да          | Папка для сгенерированных `.sql` (относительный) |
| `target`  | string | нет         | Префикс пути в `COPY ... FROM` (например, `/data` для Docker-монтирования). Может быть пустым — тогда путь в `COPY` будет просто именем файла. |

Дефолтный конфиг (создаётся утилитой при первом запуске):

```json
{
    "csv": "",
    "sql": "",
    "target": ""
}
```

## Поведение по конфигу

- **`csv_migrate_config.json` отсутствует** → создать с дефолтными пустыми
  значениями, вывести сообщение пользователю, завершить работу (exit 0).
  На повторном запуске пользователь должен заполнить поля.
- **Конфиг есть, но `csv` или `sql` пустые** → вывести ошибку
  «поля `csv` и `sql` обязательны (поле `target` может быть пустым)»,
  завершить с ненулевым кодом.
- **Конфиг есть, все три поля пустые** → то же поведение, что и выше
  (пустой `target` допустим; пустые `csv`/`sql` — нет).
- **Указанная папка (`csv` или `sql`) не существует на диске** →
  создать рекурсивно (`mkdir -p`) и продолжить работу.
- **Папка `csv` существует, но в ней нет `.csv`-файлов** → вывести
  NOTICE, ничего не генерировать, exit 0.

## Алгоритм работы

### Шаг 1. Очистка папки `sql`

Удалить в папке `sql` все файлы, оканчивающиеся на `_CSV.sql`
(только этот паттерн — никакой другой контент не трогаем).
Очистка **общая для обоих режимов** (и обычного, и `temp_table`),
поэтому запуск в любом режиме сносит ранее сгенерированные файлы
любого режима.

### Шаг 2. Сканирование папки `csv`

Пройти по всем файлам в папке `csv` **без рекурсии** (все CSV лежат
в одной плоской папке). Имя каждого CSV-файла должно строго
соответствовать формату:

```
<N>.<TABLE_NAME>.csv
```

где `<N>` — положительное целое число (без ведущих нулей: `1`, `2`, ..., `10`, `11`),
а `<TABLE_NAME>` — имя таблицы.

**!!! ВАЖНО !!!** — эти правила критичны для корректной работы:

- Если имя файла **не соответствует** формату `<N>.<TABLE_NAME>.csv`
  (нет числового префикса, не `.csv`, посторонние символы, ведущие нули
  в `<N>`) — **ошибка, exit ≠ 0** с указанием имени файла.
- Если найдено **два и более файлов с одинаковым `<N>`** —
  **ошибка, exit ≠ 0** с указанием конфликтующих имён.
- Сортировка файлов по `<N>` **не выполняется** — порядок обработки
  соответствует порядку обхода (`os.ReadDir`); пользователь контролирует
  порядок через имена файлов.
- `<N>` в выходном `.sql`-файле берётся **ровно из имени входного CSV**.

Для каждого валидного файла вычислить:

- `table` = часть имени файла после `<N>.` и до `.csv`, в **lowercase**.
  Пример: `1.Blogs.csv` → `blogs`.
- `columns` = первая строка CSV как есть (простой `strings.Split(line, ",")`,
  без поддержки quoted-полей с запятыми внутри — предполагается, что
  заголовки CSV чистые и точно соответствуют именам колонок в таблице).
  Колонки склеиваются через запятую.
- `path` = `target` + (если `target` непустой, добавить `/`) + `<filename>`.
  Нормализация слэша: между `target` и именем файла всегда ровно один `/`,
  независимо от того, есть ли слэш на конце `target`.
  Примеры:
  - `target = "/data"`, файл `1.blogs.csv` → `/data/1.blogs.csv`
  - `target = "/data/"`, файл `1.blogs.csv` → `/data/1.blogs.csv`
  - `target = ""`, файл `1.blogs.csv` → `1.blogs.csv`
- `ts` = текущее локальное время в формате `YYYYMMDDHHMMSS`.
  Пример: `20260716130027`.
- `index` = `<N>` из имени входного файла, как есть (без ведущих нулей).
  Пример: для `1.blogs.csv` → `1`, для `10.users.csv` → `10`.
- `basename_upper` = часть имени файла после `<N>.` и до `.csv`,
  в **UPPERCASE**.
  Пример: `1.blogs.csv` → `BLOGS`.
- Имя выходного `.sql`-файла = `<ts><index>_<basename_upper>_CSV.sql`.
  Пример для `1.blogs.csv`: `202607161300271_BLOGS_CSV.sql`.
  Пример для `10.users.csv`: `2026071613002710_USERS_CSV.sql`.

### Шаг 3. Запись `.sql`-файла

Содержимое файла зависит от режима (флаг `--temp-table`).

#### Шаг 3a. Обычный режим (без `--temp-table`)

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

Где:

- `<table>` — вычисленное имя таблицы (lowercase basename);
- `<columns>` — склеенный через запятую список колонок из заголовка CSV;
- `<path>` — вычисленный путь к CSV (с учётом `target`);
- `<filename>` — оригинальное имя CSV-файла (для сообщения в NOTICE).

#### Шаг 3b. Режим `temp_table` (с `--temp-table`)

Импорт идёт через временную таблицу `temp_csv_import` (создаётся с
колонками типа `TEXT`), затем строится `INSERT ... ON CONFLICT`
по `PRIMARY KEY` (приоритет) или `UNIQUE`-индексу целевой таблицы,
либо простой `INSERT` если уникальных ключей нет.

```sql
DO $$
DECLARE
    -- === ЭТИ ТРИ ПЕРЕМЕННЫЕ ПОДСТАВЛЯЕТ ГЕНЕРАТОР ===
    target_tbl  TEXT := '<table>';                       -- Имя таблицы
    columns_lst TEXT := '<columns>';                     -- Колонки из CSV через запятую
    csv_path    TEXT := '<path>';                        -- Путь к CSV-файлу
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
```

**Поведение EXCEPTION в `temp_table`-режиме:** ловится `undefined_file`
на шаге 3 (импорт CSV). При срабатывании — `RAISE NOTICE` + `RETURN`
(выход из `DO`-блока), UPSERT не выполняется. Это отличается от
обычного режима, где EXCEPTION оборачивает весь `COPY`.

## Эталонный пример (обычный режим)

Вход: `csv_source/1.blogs.csv`:

```
id,title,description,preview,preview_small,show_on_main,url,article,views,user_blogs
11111111-1111-1111-1111-111111111111,Тестовая статья,Описание статьи,/assets/blog/preview.jpg,/assets/blog/preview_small.jpg,false,test-article-1,# Тестовая статья про гранит и качество натурального камня оптом дешево,3,11111111-1111-1111-1111-111111111111
```

Выход: `sql_target/202607161300271_BLOGS_CSV.sql`:

```sql
DO $$
BEGIN

    BEGIN
        COPY blogs (id,title,description,preview,preview_small,show_on_main,url,article,views,user_blogs)
        FROM '/data/1.blogs.csv' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл 1.blogs.csv не найден, пропускаем импорт данных.';
    END;


END $$;
```

## Эталонный пример (режим `temp_table`)

Вход: `csv_source/7.promocodes.csv`:

```
promocode,discount,discount_type
SUMMER10,10,percent
WINTER20,20,percent
```

Команда: `csv_migrate_util --temp-table`

Выход: `sql_target/202607161300277_PROMOCODES_CSV.sql`:

```sql
DO $$
DECLARE
    -- === ЭТИ ТРИ ПЕРЕМЕННЫЕ ПОДСТАВЛЯЕТ ГЕНЕРАТОР ===
    target_tbl  TEXT := 'promocodes';                       -- Имя таблицы
    columns_lst TEXT := 'promocode,discount,discount_type'; -- Колонки из CSV через запятую
    csv_path    TEXT := '/data/7.promocodes.csv';           -- Путь к CSV-файлу
    -- ====================================================

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
```

## Ограничения и допущения

- CSV-парсинг — наивный: одна строка заголовка, разделитель `,`,
  без поддержки quoted-полей и переносов строк внутри ячеек.
  Заголовок должен точно соответствовать именам колонок в целевой таблице.
- Рекурсивного обхода `csv`-папки нет — все файлы лежат в одном уровне.
- Входной CSV-файл должен иметь имя `<N>.<TABLE_NAME>.csv`, где `<N>` —
  положительное целое число без ведущих нулей. Имя без числового
  префикса или с дублирующимся `<N>` — **ошибка** (exit ≠ 0).
- Суффикс `_CSV` в имени выходного файла — фиксированный,
  чтобы шаг очистки мог точно находить ранее сгенерированные файлы.
- Имя sql-файла **одинаковое в обоих режимах**; отличается только
  содержимое. Шаг очистки `*_CSV.sql` затрагивает файлы обоих режимов.
- `target` используется только как префикс в `COPY ... FROM`,
  на диске эта папка не проверяется и не создаётся —
  предполагается, что она уже смонтирована в целевом окружении
  (например, в Docker-контейнере СУБД).
- Время в имени файла — локальное (`time.Now().Format("20060102150405")`).
- Режим `temp_table` рассчитан на PostgreSQL: использует `pg_index`,
  `pg_attribute`, типы `TEXT`, `ON CONFLICT ... DO UPDATE/UPSERT`,
  `EXCEPTION WHEN undefined_file`. На других СУБД работать не будет.
