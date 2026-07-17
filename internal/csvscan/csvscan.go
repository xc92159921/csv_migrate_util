package csvscan

import (
	"fmt"
	"strconv"
	"strings"
)

// Entry — распарсенное имя CSV-файла формата `<N>.<TABLE_NAME>.csv`.
type Entry struct {
	Filename string // полное имя файла, например "1.blogs.csv"
	Index    string // N как строка (без ведущих нулей), например "1" или "10"
	Base     string // TABLE_NAME в исходном регистре, например "blogs"
}

// ParseAll парсит имена файлов в формат <N>.<TABLE_NAME>.csv.
// При нарушении формата возвращает ошибку с указанием имени файла.
func ParseAll(filenames []string) ([]Entry, error) {
	out := make([]Entry, 0, len(filenames))
	for _, name := range filenames {
		// Требуем строго ".csv" в нижнем регистре — иначе ошибка.
		if !strings.HasSuffix(name, ".csv") {
			return nil, fmt.Errorf("файл %q не соответствует формату <N>.<TABLE_NAME>.csv", name)
		}
		stem := strings.TrimSuffix(name, ".csv")
		// stem должен иметь ровно одну точку-разделитель между <N> и TABLE_NAME.
		dot := strings.Index(stem, ".")
		if dot <= 0 || dot == len(stem)-1 {
			return nil, fmt.Errorf("файл %q не соответствует формату <N>.<TABLE_NAME>.csv", name)
		}
		if strings.Contains(stem[dot+1:], ".") {
			return nil, fmt.Errorf("файл %q не соответствует формату <N>.<TABLE_NAME>.csv (TABLE_NAME не должен содержать '.')", name)
		}

		nStr := stem[:dot]
		base := stem[dot+1:]

		// <N> — положительное целое без знака и без ведущих нулей.
		if !isPositiveIntNoLeadingZeros(nStr) {
			return nil, fmt.Errorf("файл %q не соответствует формату <N>.<TABLE_NAME>.csv: <N>=%q должен быть положительным целым без ведущих нулей", name, nStr)
		}

		out = append(out, Entry{
			Filename: name,
			Index:    nStr,
			Base:     base,
		})
	}
	return out, nil
}

// CheckUniqueIndexes проверяет уникальность <N> среди всех файлов.
// При конфликте — ошибка с указанием конфликтующих имён.
func CheckUniqueIndexes(entries []Entry) error {
	seen := make(map[string]string, len(entries)) // index -> первое имя с таким index
	for _, e := range entries {
		if first, ok := seen[e.Index]; ok {
			return fmt.Errorf("обнаружены файлы с одинаковым <N>=%s: %q и %q", e.Index, first, e.Filename)
		}
		seen[e.Index] = e.Filename
	}
	return nil
}

// isPositiveIntNoLeadingZeros проверяет, что s — положительное целое
// без знака '+' и без ведущих нулей. "0", "01" и "001" — отвергаются.
func isPositiveIntNoLeadingZeros(s string) bool {
	if s == "" {
		return false
	}
	if len(s) > 1 && s[0] == '0' {
		return false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return false
	}
	return n > 0
}
