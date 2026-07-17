package iofs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// ReadHeader читает первую строку CSV-файла и возвращает её как строку
// колонок, склеенных через запятую. Наивный сплит — без поддержки quoted-полей.
func ReadHeader(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// увеличим буфер на случай длинных заголовков
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("файл пустой: %s", path)
	}
	header := scanner.Text()
	parts := strings.Split(header, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Join(parts, ","), nil
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
