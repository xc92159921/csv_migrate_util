package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xc92159921/csv_migrate_util/internal/config"
	"github.com/xc92159921/csv_migrate_util/internal/csvscan"
	"github.com/xc92159921/csv_migrate_util/internal/iofs"
	"github.com/xc92159921/csv_migrate_util/internal/render"
	"github.com/spf13/cobra"
)

// rootCmd — единственная команда утилиты. Утилита делает ровно одно:
// генерирует SQL-файлы из CSV в режиме --copy (INSERT ... ON CONFLICT (id)
// DO UPDATE, данные CSV встраиваются прямо в SQL как литералы VALUES).
var rootCmd = &cobra.Command{
	Use:   "csv_migrate_util",
	Short: "Генерация SQL-миграций из CSV-файлов (режим --copy)",
	Long:  "Утилита для генерации PostgreSQL-SQL из CSV-файлов формата <N>.<TABLE_NAME>.csv.\nРежим работы один — --copy: INSERT ... ON CONFLICT (id) DO UPDATE, данные CSV\nвстраиваются в SQL как литералы VALUES.",
	RunE:  run,
}

// Execute — точка входа, вызывается из main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadOrCreate(configFileName())
	if err != nil {
		return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	if cfg.CSV == "" || cfg.SQL == "" {
		return fmt.Errorf("поля `csv` и `sql` обязательны (поле `target` может быть пустым)")
	}

	if err := os.MkdirAll(cfg.CSV, 0o755); err != nil {
		return fmt.Errorf("не удалось создать папку csv (%s): %w", cfg.CSV, err)
	}
	if err := os.MkdirAll(cfg.SQL, 0o755); err != nil {
		return fmt.Errorf("не удалось создать папку sql (%s): %w", cfg.SQL, err)
	}

	// Шаг 1. Очистка папки sql.
	if err := iofs.CleanSQLDir(cfg.SQL); err != nil {
		return fmt.Errorf("ошибка очистки папки sql: %w", err)
	}

	// Шаг 2. Сканирование и валидация CSV.
	rawFiles, err := iofs.ListCSVFiles(cfg.CSV)
	if err != nil {
		return fmt.Errorf("ошибка сканирования папки csv: %w", err)
	}
	if len(rawFiles) == 0 {
		log.Printf("NOTICE: в папке %s не найдено .csv-файлов, ничего не генерируем", cfg.CSV)
		return nil
	}

	entries, err := csvscan.ParseAll(rawFiles)
	if err != nil {
		return err
	}
	if err := csvscan.CheckUniqueIndexes(entries); err != nil {
		return err
	}

	// Шаг 3. Запись .sql-файлов в режиме --copy.
	ts := time.Now().Format("20060102150405")
	for _, e := range entries {
		table := strings.ToLower(e.Base)
		basenameUpper := strings.ToUpper(e.Base)

		// Считываем заголовок CSV (с поддержкой кавычек)
		columns, err := iofs.ReadHeader(filepath.Join(cfg.CSV, e.Filename))
		if err != nil {
			return fmt.Errorf("не удалось прочитать заголовок %s: %w", e.Filename, err)
		}

		// Режим --copy: колонка id обязательна — это ключ UPSERT.
		hasID := false
		for _, c := range columns {
			if strings.TrimSpace(c) == "id" {
				hasID = true
				break
			}
		}
		if !hasID {
			return fmt.Errorf("файл %s: режим --copy требует колонку `id` в заголовке CSV", e.Filename)
		}

		rows, err := iofs.ReadDataRows(filepath.Join(cfg.CSV, e.Filename), len(columns))
		if err != nil {
			return fmt.Errorf("не удалось прочитать данные %s: %w", e.Filename, err)
		}

		content := render.CopyInlineSQL(table, strings.Join(columns, ","), rows)

		outName := fmt.Sprintf("%s%s_%s_CSV.sql", ts, e.Index, basenameUpper)
		outFile := filepath.Join(cfg.SQL, outName)

		if err := os.WriteFile(outFile, []byte(content), 0o644); err != nil {
			return fmt.Errorf("не удалось записать %s: %w", outFile, err)
		}
		log.Printf("сгенерирован %s", outFile)
	}
	return nil
}

// configFileName возвращает имя файла конфига. Конфиг ищется в cwd.
func configFileName() string { return "csv_migrate_config.json" }
