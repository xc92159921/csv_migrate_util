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

var tempTable bool

// rootCmd — единственная команда утилиты. Вся логика собрана здесь,
// потому что операция одна — сгенерировать SQL-файлы из CSV.
var rootCmd = &cobra.Command{
	Use:   "csv_migrate_util",
	Short: "Генерация SQL-миграций из CSV-файлов",
	Long:  "Утилита для генерации PostgreSQL-SQL из CSV-файлов формата <N>.<TABLE_NAME>.csv.",
	RunE:  run,
}

func init() {
	rootCmd.Flags().BoolVarP(&tempTable, "temp-table", "t", false,
		"Сгенерировать SQL в режиме temp_table (импорт через временную таблицу + UPSERT по PK/UNIQUE)")
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

	// Шаг 1. Очистка папки sql — общая для обоих режимов.
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

	// Шаг 3. Запись .sql-файлов.
	ts := time.Now().Format("20060102150405")
	for _, e := range entries {
		table := strings.ToLower(e.Base)
		basenameUpper := strings.ToUpper(e.Base)

		columns, err := iofs.ReadHeader(filepath.Join(cfg.CSV, e.Filename))
		if err != nil {
			return fmt.Errorf("не удалось прочитать заголовок %s: %w", e.Filename, err)
		}

		outPath := iofs.BuildCopyPath(cfg.Target, e.Filename)
		outName := fmt.Sprintf("%s%s_%s_CSV.sql", ts, e.Index, basenameUpper)
		outFile := filepath.Join(cfg.SQL, outName)

		var content string
		if tempTable {
			content = render.TempTableSQL(table, columns, outPath, e.Filename)
		} else {
			content = render.NormalSQL(table, columns, outPath, e.Filename)
		}

		if err := os.WriteFile(outFile, []byte(content), 0o644); err != nil {
			return fmt.Errorf("не удалось записать %s: %w", outFile, err)
		}
		log.Printf("сгенерирован %s", outFile)
	}
	return nil
}

// configFileName возвращает имя файла конфига. Конфиг ищется в cwd.
func configFileName() string { return "csv_migrate_config.json" }
