package render

import (
	"fmt"
	"strings"
)

// InitOnlyColumn — имя необязательной служебной колонки CSV.
// Если в строке значение "1" — строка попадает в "insert-only" батч
// (ON CONFLICT (id) DO NOTHING), иначе — в обычный UPSERT-батч.
const InitOnlyColumn = "init_only"

// CopyInlineSQL рендерит SQL в режиме --copy: данные CSV встраиваются прямо
// в SQL как литералы VALUES, без COPY. Это единственный режим работы
// утилиты csv_migrate_util.
//
// Колонка id обязательна — это ключ ON CONFLICT.
//
// Опциональная колонка "init_only" управляет тем, затирается ли строка
// менеджером при повторном применении миграции:
//
//   - init_only = "1" → строка попадает в батч с ON CONFLICT (id) DO NOTHING.
//     Если id уже есть в таблице — строка молча пропускается (менеджерская
//     правка не затирается). Если id нет — строка вставляется.
//   - init_only = "0" или пусто (или колонки init_only в CSV нет) → строка
//     попадает в обычный UPSERT-батч (ON CONFLICT (id) DO UPDATE SET ...).
//
// Когда init_only присутствует, SQL-файл содержит ДВА отдельных INSERT-батча:
//
//   1) батч init_only-строк: ON CONFLICT (id) DO NOTHING;
//   2) батч обычных строк:   ON CONFLICT (id) DO UPDATE SET ...;
//
// Это нужно, потому что ON CONFLICT действует на весь INSERT, а не на
// отдельные строки: если смешать оба типа строк в одном INSERT и использовать
// DO NOTHING — обычные строки не будут UPSERT-иться; если использовать
// DO UPDATE SET — затираются менеджерские правки у init_only-строк.
//
// Контракт:
//   - columns: список имён колонок из заголовка CSV через запятую, без
//     пробелов вокруг имён. Колонка 'id' обязана присутствовать.
//     Колонка 'init_only' опциональна.
//   - rows: значения строк данных (без заголовка) в том же порядке колонок.
//     Каждая row[i] соответствует columns[i]. Пустая ячейка → SQL NULL.
//
// Экранирование: одинарная кавычка внутри значения удваивается ('').
// Типы колонок в SQL не кастуются — PG сам приведёт TEXT-литерал к
// целевому типу колонки.
func CopyInlineSQL(table, columns string, rows [][]string) string {
	cols := splitAndTrim(columns)
	colIndex := make(map[string]int, len(cols))
	for i, c := range cols {
		colIndex[c] = i
	}

	if _, ok := colIndex["id"]; !ok {
		// Без id нет ключа ON CONFLICT. Вызывающий код должен был
		// это поймать раньше; здесь защищаемся от кривого контракта.
		return fmt.Sprintf("-- ОШИБКА: csv_migrate_util --copy требует колонку `id` в заголовке CSV для таблицы %s\n", table)
	}

	// Все колонки, идущие в INSERT/VALUES (без init_only; id остаётся,
	// чтобы вставлять новые строки с нужным ключом).
	dataCols := make([]string, 0, len(cols))
	for _, c := range cols {
		if c == InitOnlyColumn {
			continue
		}
		dataCols = append(dataCols, c)
	}
	dataColumnsJoined := strings.Join(dataCols, ",")

	// Индекс колонки init_only в исходной row (или -1, если её нет).
	initOnlyIdx := -1
	if i, ok := colIndex[InitOnlyColumn]; ok {
		initOnlyIdx = i
	}

	// Разделяем строки на init_only-батч и обычный по значению init_only.
	var initOnlyRows, normalRows [][]string
	for _, row := range rows {
		if initOnlyIdx >= 0 && initOnlyIdx < len(row) && strings.TrimSpace(row[initOnlyIdx]) == "1" {
			initOnlyRows = append(initOnlyRows, row)
		} else {
			normalRows = append(normalRows, row)
		}
	}

	// SET-часть для обычного батча: все data-колонки, КРОМЕ id.
	// id — ключ ON CONFLICT, его нельзя обновлять.
	setParts := make([]string, 0, len(dataCols))
	for _, c := range dataCols {
		if c == "id" {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("    %s = EXCLUDED.%s", c, c))
	}

	var sb strings.Builder
	header := fmt.Sprintf("-- Сгенерировано csv_migrate_util --copy для таблицы %s", table)

	// Батч 1: init_only-строки → DO NOTHING.
	if len(initOnlyRows) > 0 {
		sb.WriteString(header)
		sb.WriteString(" (init_only: ON CONFLICT (id) DO NOTHING)\n")
		writeInsert(&sb, table, dataColumnsJoined, initOnlyRows, dataCols, cols)
		sb.WriteString("ON CONFLICT (id) DO NOTHING;\n")
	}

	// Батч 2: обычные строки → DO UPDATE SET (или DO NOTHING если dataCols пуст).
	if len(normalRows) > 0 {
		if len(initOnlyRows) > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(header)
		if len(setParts) == 0 {
			sb.WriteString(" (нет data-колонок, кроме id: DO NOTHING)\n")
		} else {
			sb.WriteString("\n")
		}
		writeInsert(&sb, table, dataColumnsJoined, normalRows, dataCols, cols)
		if len(setParts) == 0 {
			sb.WriteString("ON CONFLICT (id) DO NOTHING;\n")
		} else {
			sb.WriteString("ON CONFLICT (id) DO UPDATE SET\n")
			sb.WriteString(strings.Join(setParts, ",\n"))
			sb.WriteString(";\n")
		}
	}

	return sb.String()
}

// writeInsert пишет "INSERT INTO <table> (<cols>) VALUES\n    (...), (...)\n"
// без завершающей ON CONFLICT-части — её добавляет вызывающий код.
//
// dataCols — список имён колонок, идущих в INSERT (без id, без init_only),
// в нужном порядке. allCols — полный список имён колонок CSV; используется
// чтобы по имени колонки найти её индекс в row. Используется только
// внутри CopyInlineSQL.
func writeInsert(sb *strings.Builder, table, dataColumns string, rows [][]string, dataCols, allCols []string) {
	sb.WriteString("INSERT INTO ")
	sb.WriteString(table)
	sb.WriteString(" (")
	sb.WriteString(dataColumns)
	sb.WriteString(") VALUES\n")

	colIndex := make(map[string]int, len(allCols))
	for i, c := range allCols {
		colIndex[c] = i
	}

	for i, row := range rows {
		sb.WriteString("    (")
		for j, colName := range dataCols {
			if j > 0 {
				sb.WriteString(", ")
			}
			idx, ok := colIndex[colName]
			var cell string
			if ok && idx < len(row) {
				cell = row[idx]
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
}

// splitAndTrim разбивает строку "a,b,c" на ["a","b","c"] с TrimSpace
// каждого элемента.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}
