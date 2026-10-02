/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "path/filepath"
    "regexp"
    "github.com/google/uuid"
    "github.com/spf13/cobra"
    "github.com/xc92159921/csv_migrate_util/internal/config"
)

// uuidFillCmd represents the uuidFill command
var uuidFillCmd = &cobra.Command{
    Use:   "uuidFill",
    Short: "Заменить {uuid} в CSV на новые UUID",
    Long: `Проходит по всем CSV-файлам, указанным в конфиге, и заменяет каждое вхождение {uuid} на уникальный uuid.New(). Сохраняет файлы.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        // Load configuration
        cfg, err := config.LoadOrCreate(configFileName())
        if err != nil {
            return fmt.Errorf("ошибка загрузки конфигурации: %w", err)
        }
        if cfg.CSV == "" {
            return fmt.Errorf("поле `csv` в конфиге не задано")
        }
        // Ensure CSV directory exists
        if err := os.MkdirAll(cfg.CSV, 0o755); err != nil {
            return fmt.Errorf("не удалось создать папку csv (%s): %w", cfg.CSV, err)
        }
        // List CSV files
        entries, err := os.ReadDir(cfg.CSV)
        if err != nil {
            return fmt.Errorf("не удалось прочитать папку %s: %w", cfg.CSV, err)
        }
        re := regexp.MustCompile(`\{uuid\}`)
        for _, e := range entries {
            if e.IsDir() {
                continue
            }
            ext := filepath.Ext(e.Name())
            if ext != ".csv" && ext != ".CSV" {
                continue
            }
            fullPath := filepath.Join(cfg.CSV, e.Name())
            data, err := ioutil.ReadFile(fullPath)
            if err != nil {
                return fmt.Errorf("не удалось прочитать %s: %w", fullPath, err)
            }
            // Replace each {uuid} with a newly generated UUID
            replaced := re.ReplaceAllFunc(data, func(_ []byte) []byte {
                return []byte(uuid.New().String())
            })
            if err := ioutil.WriteFile(fullPath, replaced, 0o644); err != nil {
                return fmt.Errorf("не удалось записать %s: %w", fullPath, err)
            }
            log.Printf("processed %s", e.Name())
        }
        return nil
    },
}

func init() {
    rootCmd.AddCommand(uuidFillCmd)
}
