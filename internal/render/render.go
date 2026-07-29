package render

import (
	"fmt"
	"strings"
)

// CopyInlineSQL рендерит SQL в режиме copy (без COPY: данные CSV
// встраиваются прямо в SQL в виде литералов VALUES). На каждую таблицу
// генерируется один INSERT ... VALUES (...), (...) ... ON CONFLICT (id)
// DO UPDATE SET ... по всем колонкам кроме id.
//
// Контракт:
//   - columns: список колонок из заголовка CSV через запятую, без пробелов
//     вокруг имён. Колонка 'id' обязана присутствовать — это ключ UPSERT.
//   - rows: значения строк данных (без заголовка) в том же порядке колонок.
//     Пустая ячейка трактуется как SQL NULL.
//
// Экранирование: одинарная кавычка внутри значения удваивается ('').
// Типы колонок не кастуются — PG сам приведёт TEXT-литерал к целевому типу
// (то же поведение, что и в режиме upsert при COPY во временную таблицу).
func CopyInlineSQL(table, columns string, rows [][]string) string {
	cols := strings.Split(columns, ",")

	// Идемпотентный SET: обновляем все колонки, кроме 'id'.
	setParts := make([]string, 0, len(cols))
	for _, c := range cols {
		c = strings.TrimSpace(c)
		if c == "" || c == "id" {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("    %s = EXCLUDED.%s", c, c))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("-- Сгенерировано csv_migrate_util --copy для таблицы %s\n", table))
	sb.WriteString("INSERT INTO ")
	sb.WriteString(table)
	sb.WriteString(" (")
	sb.WriteString(columns)
	sb.WriteString(") VALUES\n")

	for i, row := range rows {
		sb.WriteString("    (")
		for j, cell := range row {
			if j > 0 {
				sb.WriteString(", ")
			}
			if cell == "" {
				sb.WriteString("NULL")
			} else {
				sb.WriteString("'")
				sb.WriteString(strings.ReplaceAll(cell, "'", "''"))
				sb.WriteString("'")
			}
		}
		sb.WriteString(")")
		if i < len(rows)-1 {
			sb.WriteString(",\n")
		} else {
			sb.WriteString("\n")
		}
	}

	if len(setParts) == 0 {
		// в CSV только id (или он один) — нечего обновлять, DO NOTHING
		sb.WriteString("ON CONFLICT (id) DO NOTHING;\n")
	} else {
		sb.WriteString("ON CONFLICT (id) DO UPDATE SET\n")
		sb.WriteString(strings.Join(setParts, ",\n"))
		sb.WriteString(";\n")
	}

	return sb.String()
}

// NormalSQL рендерит SQL в обычном режиме (прямая COPY).
//
// Формат фиксированный по agents.md — отступы и пустые строки как в эталоне.
func NormalSQL(table, columns, copyPath, filename string) string {
	return fmt.Sprintf(`DO $$
BEGIN

    BEGIN
        COPY %s (%s)
        FROM '%s' 
        DELIMITER ',' CSV HEADER;
    EXCEPTION 
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл %s не найден, пропускаем импорт данных.';
    END;


END $$;
`, table, columns, copyPath, filename)
}

// TempTableSQL рендерит SQL в режиме temp_table (импорт через временную
// таблицу + UPSERT по PK/UNIQUE).
//
// Формат фиксированный по agents.md — отступы и пустые строки как в эталоне.
func TempTableSQL(table, columns, copyPath, filename string) string {
	return fmt.Sprintf(`DO $$
DECLARE
    -- === ЭТИ ТРИ ПЕРЕМЕННЫЕ ПОДСТАВЛЯЕТ ГЕНЕРАТОР ===
    target_tbl  TEXT := '%s';                       -- Имя таблицы
    columns_lst TEXT := '%s';                     -- Колонки из CSV через запятую
    csv_path    TEXT := '%s';                        -- Путь к CSV-файлу
    -- =================================================

    temp_tbl_fields TEXT;
    conflict_cols   TEXT;
    update_set      TEXT;
    final_sql       TEXT;
BEGIN
    -- 1. Превращаем список 'col1,col2' в определение для таблицы: 'col1 TEXT, col2 TEXT'
    SELECT string_agg(format('%%I TEXT', trim(col)), ', ')
    INTO temp_tbl_fields
    FROM unnest(string_to_array(columns_lst, ',')) AS col;

    -- 2. Создаем временную таблицу, где все типы TEXT
    EXECUTE format('CREATE TEMP TABLE temp_csv_import (%%s) ON COMMIT DROP', temp_tbl_fields);

    -- 3. Безопасно импортируем CSV-данные во временную таблицу
    BEGIN
        EXECUTE format('
            COPY temp_csv_import (%%s)
            FROM %%L
            DELIMITER '','' CSV HEADER',
            columns_lst, csv_path
        );
    EXCEPTION
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл %% не найден, пропускаем импорт.', csv_path;
            RETURN;
    END;

    -- 4. Ищем уникальный ключ таблицы (PRIMARY KEY в приоритете, иначе UNIQUE)
    SELECT string_agg(format('%%I', att.attname), ', ')
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
        SELECT string_agg(format('%%1$I = EXCLUDED.%%1$I', trim(col)), ', ')
        INTO update_set
        FROM unnest(string_to_array(columns_lst, ',')) AS col
        WHERE trim(col) NOT IN (
            SELECT trim(c) FROM unnest(string_to_array(conflict_cols, ',')) c
        );

        IF update_set IS NULL OR update_set = '' THEN
            final_sql := format('
                INSERT INTO %%1$I (%%2$s)
                SELECT %%2$s FROM temp_csv_import
                ON CONFLICT (%%3$s) DO NOTHING',
                target_tbl, columns_lst, conflict_cols
            );
        ELSE
            final_sql := format('
                INSERT INTO %%1$I (%%2$s)
                SELECT %%2$s FROM temp_csv_import
                ON CONFLICT (%%3$s)
                DO UPDATE SET %%4$s',
                target_tbl, columns_lst, conflict_cols, update_set
            );
        END IF;
    ELSE
        -- 6. Если у таблицы вообще нет уникальных ключей — просто дописываем
        final_sql := format('
            INSERT INTO %%1$I (%%2$s)
            SELECT %%2$s FROM temp_csv_import',
            target_tbl, columns_lst
        );
    END IF;

    -- 7. Выполняем один итоговый запрос
    EXECUTE final_sql;

    RAISE NOTICE 'Импорт в таблицу %% успешно выполнен (UPSERT).', target_tbl;
END $$;
`, table, columns, copyPath)
}

// UpsertSQL рендерит SQL в режиме upsert (импорт через временную таблицу +
// UPSERT по колонке id). По условиям эксплуатации колонка id всегда
// присутствует в CSV и всегда уникальна в целевой таблице.
//
// Шаблон максимально простой: temp-таблица → COPY → INSERT ... ON CONFLICT.
// Никакой динамики внутри SQL — Go-генератор заранее знает список колонок и
// подставляет готовые куски в плейсхолдеры %s. Колонка конфликта захардкожена
// как 'id' (как требует задача). В SET-часть попадают все колонки CSV кроме 'id'.
//
// Формат фиксированный по agents.md — отступы и пустые строки как в эталоне.
func UpsertSQL(table, columns, copyPath string) string {
	// Разбираем список колонок CSV, чтобы сформировать готовые куски SQL.
	cols := strings.Split(columns, ",")
	tblFields := make([]string, 0, len(cols)) // для CREATE TEMP TABLE: "id" TEXT, "url" TEXT, ...
	copyCols := make([]string, 0, len(cols))  // для COPY/INSERT: "id", "url", ...
	setCols := make([]string, 0, len(cols))   // для SET: "url" = EXCLUDED."url", ...
	for _, c := range cols {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		tblFields = append(tblFields, fmt.Sprintf(`%q TEXT`, c))
		copyCols = append(copyCols, fmt.Sprintf(`%q`, c))
		if c != "id" {
			setCols = append(setCols, fmt.Sprintf(`%q = EXCLUDED.%q`, c, c))
		}
	}
	tblFieldsSQL := strings.Join(tblFields, ", ")
	copyColsSQL := strings.Join(copyCols, ", ")
	setColsSQL := strings.Join(setCols, ", ")

	return fmt.Sprintf(`DO $$
DECLARE
    target_tbl  TEXT := '%s';
    columns_lst TEXT := '%s';
    csv_path    TEXT := '%s';
BEGIN
    -- 1. Создаём временную таблицу, все колонки TEXT
    EXECUTE 'CREATE TEMP TABLE temp_csv_import (%s) ON COMMIT DROP';

    -- 2. Импортируем CSV
    BEGIN
        EXECUTE 'COPY temp_csv_import (%s) FROM ' || quote_literal(csv_path) || ' DELIMITER '','' CSV HEADER';
    EXCEPTION
        WHEN undefined_file THEN
            RAISE NOTICE 'Файл %% не найден, пропускаем импорт.', csv_path;
            RETURN;
    END;

    -- 3. UPSERT по id
    EXECUTE 'INSERT INTO ' || quote_ident(target_tbl) || ' (%s)
            SELECT %s FROM temp_csv_import
            ON CONFLICT (id) DO UPDATE SET %s';

    RAISE NOTICE 'Импорт в таблицу %% успешно выполнен (UPSERT по id).', target_tbl;
END $$;
`,
		table, columns, copyPath,
		tblFieldsSQL,
		copyColsSQL,
		copyColsSQL, copyColsSQL, setColsSQL,
	)
}

