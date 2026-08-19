package render

import (
	"fmt"
	"strings"
)

// CopyInlineSQL рендерит SQL в режиме --copy (без COPY: данные CSV
// встраиваются прямо в SQL в виде литералов VALUES). На каждую таблицу
// генерируется один INSERT ... VALUES (...), (...) ... ON CONFLICT (id)
// DO UPDATE SET ... по всем колонкам кроме id.
//
// Это единственный режим работы утилиты csv_migrate_util.
//
// Контракт:
//   - columns: список колонок из заголовка CSV через запятую, без пробелов
//     вокруг имён. Колонка 'id' обязана присутствовать — это ключ UPSERT.
//   - rows: значения строк данных (без заголовка) в том же порядке колонок.
//     Пустая ячейка трактуется как SQL NULL.
//
// Экранирование: одинарная кавычка внутри значения удваивается ('').
// Типы колонок не кастуются — PG сам приведёт TEXT-литерал к целевому типу.
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
