package cmd

import (
    "fmt"
    "log"
    "os"
    "github.com/spf13/cobra"
    "github.com/xc92159921/csv_migrate_util/internal/config"
    "github.com/xc92159921/csv_migrate_util/internal/iofs"
)

// cleanCmd removes all files ending with "_CSV.sql" from the SQL directory.
var cleanCmd = &cobra.Command{
    Use:   "clean",
    Short: "Удалить *.sql файлы, оканчивающиеся на _CSV.sql в папке sql",
    Long:  "Команда удаляет из директории, указанной в конфиге (поле `sql`), все файлы, имя которых заканчивается на _CSV.sql. Другие файлы и подпапки не затрагиваются.",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := config.LoadOrCreate(configFileName())
        if err != nil {
            return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
        }
        if cfg.SQL == "" {
            return fmt.Errorf("поле `sql` в конфиге не задано")
        }
        // Ensure the directory exists so CleanSQLDir can operate without error.
        if err := os.MkdirAll(cfg.SQL, 0o755); err != nil {
            return fmt.Errorf("не удалось создать папку sql (%s): %w", cfg.SQL, err)
        }
        if err := iofs.CleanSQLDir(cfg.SQL); err != nil {
            return fmt.Errorf("ошибка очистки папки sql: %w", err)
        }
        log.Printf("✓ Очищена директория %s от файлов *_CSV.sql", cfg.SQL)
        return nil
    },
}

func init() {
    rootCmd.AddCommand(cleanCmd)
}
