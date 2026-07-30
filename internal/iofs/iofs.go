package iofs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"encoding/csv"
)

// CleanSQLDir удаляет в папке dir все файлы, оканчивающиеся на "_CSV.sql".
// Другой контент не трогается. Подпапки игнорируются.
func CleanSQLDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, "_CSV.sql") {
			full := filepath.Join(dir, name)
			if err := os.Remove(full); err != nil {
				return fmt.Errorf("удаление %s: %w", full, err)
			}
		}
	}
	return nil
}

// ListCSVFiles возвращает имена .csv-файлов в папке dir в порядке os.ReadDir
// (без сортировки). Подпапки игнорируются.
func ListCSVFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".csv") {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// ReadHeader читает первую строку CSV-файла и возвращает список колонок,
// разбитых по запятой с поддержкой кавычек и пробелов вокруг имён.
// Пример: '"id","h1","title"' → []string{"id", "h1", "title"}
func ReadHeader(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("файл пустый: %s", path)
	}

	line := scanner.Text()

	// Используем standard library encoding/csv для корректного парсинга
	hReader := csv.NewReader(strings.NewReader(line))
	rows, err := hReader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("файл пустый: %s", path)
	}

	parts := rows[0]
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts, nil
}

// ReadDataRows читает все строки данных CSV (без заголовка) и возвращает
// их в виде слайсов значений колонок. Пустые строки пропускаются.
func ReadDataRows(path string, expectedCols int) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)

	// Считываем заголовок
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("файл пустый: %s", path)
	}

	// Парсим заголовок для определения количества колонок
	hReader := csv.NewReader(strings.NewReader(scanner.Text()))
	hRows, err := hReader.ReadAll()
	if err != nil {
		return nil, err
	}
	actualCols := len(hRows[0])

	if actualCols == 0 {
		actualCols = expectedCols
	}

	var rows [][]string
	lineNo := 1
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if line == "" {
			continue
		}
		rowsReader := csv.NewReader(strings.NewReader(line))
		row, err := rowsReader.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("файл %s: строка %d: %w", path, lineNo, err)
		}
		if len(row) == 0 {
			continue
		}
		for i, c := range row[0] {
			// Убираем кавычки и пробелы
			if strings.HasPrefix(c, "'") && strings.HasSuffix(c, "'") {
				row[0][i] = strings.Trim(c, "'")
			}
			row[0][i] = strings.TrimSpace(row[0][i])
		}
		if len(row[0]) != actualCols {
			return nil, fmt.Errorf("файл %s: строка %d содержит %d колонок, ожидалось %d", path, lineNo, len(row[0]), actualCols)
		}
		rows = append(rows, row[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

// BuildCopyPath склеивает target и filename через один '/'. Если target
// пуст — возвращается просто filename.
func BuildCopyPath(target, filename string) string {
	target = strings.TrimRight(target, "/")
	if target == "" {
		return filename
	}
	return target + "/" + filename
}