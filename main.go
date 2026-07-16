package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const configFileName = "csv_migrate_config.json"

type Config struct {
	CSV    string `json:"csv"`
	SQL    string `json:"sql"`
	Target string `json:"target"`
}

// csvEntry — распарсенное имя CSV-файла формата `<N>.<TABLE_NAME>.csv`.
type csvEntry struct {
	filename string // полное имя файла, например "1.blogs.csv"
	index    string // N как строка (без ведущих нулей), например "1" или "10"
	base     string // TABLE_NAME в исходном регистре, например "blogs"
}

func main() {
	cfg, err := loadOrCreateConfig()
	if err != nil {
		log.Fatalf("ошибка загрузки конфигурации: %v", err)
	}

	if cfg.CSV == "" || cfg.SQL == "" {
		log.Fatalf("поля `csv` и `sql` обязательны (поле `target` может быть пустым)")
	}

	if err := os.MkdirAll(cfg.CSV, 0o755); err != nil {
		log.Fatalf("не удалось создать папку csv (%s): %v", cfg.CSV, err)
	}
	if err := os.MkdirAll(cfg.SQL, 0o755); err != nil {
		log.Fatalf("не удалось создать папку sql (%s): %v", cfg.SQL, err)
	}

	if err := cleanSQLDir(cfg.SQL); err != nil {
		log.Fatalf("ошибка очистки папки sql: %v", err)
	}

	rawFiles, err := listCSVDir(cfg.CSV)
	if err != nil {
		log.Fatalf("ошибка сканирования папки csv: %v", err)
	}

	if len(rawFiles) == 0 {
		log.Printf("NOTICE: в папке %s не найдено .csv-файлов, ничего не генерируем", cfg.CSV)
		return
	}

	// Парсим и валидируем имена файлов, собираем в порядке обхода os.ReadDir.
	entries, err := parseAndValidate(rawFiles)
	if err != nil {
		log.Fatalf("%v", err)
	}

	// Проверяем уникальность <N> среди всех файлов.
	if err := checkUniqueIndexes(entries); err != nil {
		log.Fatalf("%v", err)
	}

	ts := time.Now().Format("20060102150405")
	for _, e := range entries {
		table := strings.ToLower(e.base)
		basenameUpper := strings.ToUpper(e.base)

		columns, err := readHeader(filepath.Join(cfg.CSV, e.filename))
		if err != nil {
			log.Fatalf("не удалось прочитать заголовок %s: %v", e.filename, err)
		}

		outPath := buildCopyPath(cfg.Target, e.filename)
		outName := fmt.Sprintf("%s%s_%s_CSV.sql", ts, e.index, basenameUpper)
		outFile := filepath.Join(cfg.SQL, outName)

		content := renderSQL(table, columns, outPath, e.filename)
		if err := os.WriteFile(outFile, []byte(content), 0o644); err != nil {
			log.Fatalf("не удалось записать %s: %v", outFile, err)
		}
		log.Printf("сгенерирован %s", outFile)
	}
}

func loadOrCreateConfig() (*Config, error) {
	data, err := os.ReadFile(configFileName)
	if err != nil {
		if os.IsNotExist(err) {
			defaultCfg := &Config{CSV: "", SQL: "", Target: ""}
			buf, _ := json.MarshalIndent(defaultCfg, "", "    ")
			if werr := os.WriteFile(configFileName, buf, 0o644); werr != nil {
				return nil, werr
			}
			fmt.Printf("файл %s не найден — создан с дефолтными пустыми значениями. Заполните поля `csv` и `sql` и запустите утилиту снова.\n", configFileName)
			os.Exit(0)
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("некорректный JSON в %s: %w", configFileName, err)
	}
	return &cfg, nil
}

func cleanSQLDir(dir string) error {
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

// listCSVDir возвращает имена .csv-файлов в порядке os.ReadDir (без сортировки).
func listCSVDir(dir string) ([]string, error) {
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

// parseAndValidate парсит имя файла формата `<N>.<TABLE_NAME>.csv`.
// Возвращает ошибку, если имя не соответствует формату.
func parseAndValidate(filenames []string) ([]csvEntry, error) {
	out := make([]csvEntry, 0, len(filenames))
	for _, name := range filenames {
		// Требуем строго ".csv" (lowercase) — иначе ошибка.
		// Но os.ReadDir возвращает имена как есть на диске; разрешим и ".CSV"
		// для дружелюбности? Спека говорит "строго соответствовать формату".
		// Трактую строго: расширение только ".csv" в нижнем регистре.
		// Однако часть "<N>." и сам TABLE_NAME могут быть в любом регистре.
		if !strings.HasSuffix(name, ".csv") || strings.HasSuffix(name, ".CSV") {
			return nil, fmt.Errorf("файл %q не соответствует формату <N>.<TABLE_NAME>.csv", name)
		}
		stem := strings.TrimSuffix(name, ".csv")
		// stem не должен содержать дополнительных точек (имя таблицы без точек,
		// иначе распарсить однозначно нельзя).
		dot := strings.Index(stem, ".")
		if dot <= 0 || dot == len(stem)-1 {
			return nil, fmt.Errorf("файл %q не соответствует формату <N>.<TABLE_NAME>.csv", name)
		}
		if strings.Contains(stem[dot+1:], ".") {
			return nil, fmt.Errorf("файл %q не соответствует формату <N>.<TABLE_NAME>.csv (TABLE_NAME не должен содержать '.')", name)
		}

		nStr := stem[:dot]
		base := stem[dot+1:]

		// <N> — положительное целое, без знака, без ведущих нулей.
		if !isPositiveIntNoLeadingZeros(nStr) {
			return nil, fmt.Errorf("файл %q не соответствует формату <N>.<TABLE_NAME>.csv: <N>=%q должен быть положительным целым без ведущих нулей", name, nStr)
		}

		out = append(out, csvEntry{
			filename: name,
			index:    nStr,
			base:     base,
		})
	}
	return out, nil
}

// isPositiveIntNoLeadingZeros проверяет, что s — положительное целое
// без знака '+' и без ведущих нулей. "0" и "01" и "001" — отвергаются.
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

// checkUniqueIndexes проверяет, что все <N> уникальны.
// При конфликте — ошибка с указанием конфликтующих имён.
func checkUniqueIndexes(entries []csvEntry) error {
	seen := make(map[string]string, len(entries)) // index -> первое имя с таким index
	for _, e := range entries {
		if first, ok := seen[e.index]; ok {
			return fmt.Errorf("обнаружены файлы с одинаковым <N>=%s: %q и %q", e.index, first, e.filename)
		}
		seen[e.index] = e.filename
	}
	return nil
}

func readHeader(path string) (string, error) {
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
	// наивный сплит по запятой — соответствует agents.md
	parts := strings.Split(header, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Join(parts, ","), nil
}

func buildCopyPath(target, filename string) string {
	target = strings.TrimRight(target, "/")
	if target == "" {
		return filename
	}
	return target + "/" + filename
}

func renderSQL(table, columns, copyPath, filename string) string {
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
