package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const configFileName = "csv_migrate_config.json"

type Config struct {
	CSV    string `json:"csv"`
	SQL    string `json:"sql"`
	Target string `json:"target"`
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

	csvFiles, err := scanCSVDir(cfg.CSV)
	if err != nil {
		log.Fatalf("ошибка сканирования папки csv: %v", err)
	}

	if len(csvFiles) == 0 {
		log.Printf("NOTICE: в папке %s не найдено .csv-файлов, ничего не генерируем", cfg.CSV)
		return
	}

	ts := time.Now().Format("20060102150405")
	for _, csvPath := range csvFiles {
		filename := filepath.Base(csvPath)
		base := strings.TrimSuffix(filename, filepath.Ext(filename))
		table := strings.ToLower(base)
		basenameUpper := strings.ToUpper(base)

		columns, err := readHeader(csvPath)
		if err != nil {
			log.Fatalf("не удалось прочитать заголовок %s: %v", csvPath, err)
		}

		outPath := buildCopyPath(cfg.Target, filename)
		outName := fmt.Sprintf("%s_%s_CSV.sql", ts, basenameUpper)
		outFile := filepath.Join(cfg.SQL, outName)

		content := renderSQL(table, columns, outPath, filename)
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

func scanCSVDir(dir string) ([]string, error) {
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
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out, nil
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
