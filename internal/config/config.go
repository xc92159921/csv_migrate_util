package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config — соответствует csv_migrate_config.json.
type Config struct {
	CSV    string `json:"csv"`
	SQL    string `json:"sql"`
	Target string `json:"target"`
}

// LoadOrCreate пытается прочитать path. Если файла нет — создаёт дефолтный
// конфиг с пустыми значениями, выводит сообщение и завершает процесс
// с кодом 0. Если файл есть, но не парсится — возвращает ошибку.
func LoadOrCreate(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			defaultCfg := &Config{CSV: "", SQL: "", Target: ""}
			buf, _ := json.MarshalIndent(defaultCfg, "", "    ")
			if werr := os.WriteFile(path, buf, 0o644); werr != nil {
				return nil, werr
			}
			fmt.Printf("файл %s не найден — создан с дефолтными пустыми значениями. Заполните поля `csv` и `sql` и запустите утилиту снова.\n", path)
			os.Exit(0)
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("некорректный JSON в %s: %w", path, err)
	}
	return &cfg, nil
}
