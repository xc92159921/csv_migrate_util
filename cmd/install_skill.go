package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xc92159921/csv_migrate_util/skills"
)

const skillName = "csv_migrate_util"
const skillTargetDir = ".agents/skills/csv_migrate_util"

var installSkillCmd = &cobra.Command{
	Use:   "install-skill",
	Short: "Устанавливает скилл csv_migrate_util в ~/.agents/skills/",
	Long: `Устанавливает CLI скилл csv_migrate_util для работы с миграциями в postgres.
Создаёт папку ~/.agents/skills/csv_migrate_util и копирует туда все файлы скилла.`,
	Run: func(cmd *cobra.Command, args []string) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Printf("Ошибка получения домашней директории: %v\n", err)
			return
		}

		targetDir := filepath.Join(homeDir, skillTargetDir)

		// Копируем файлы из embed скилла
		err = extractSkillFiles(skillName, targetDir)
		if err != nil {
			fmt.Printf("Ошибка копирования файлов: %v\n", err)
			return
		}

		fmt.Printf("✓ Скилл установлен в: %s\n", targetDir)
		fmt.Println("Используйте: /skill:csv_migrate_util")
		fmt.Println("Или добавьте в PATH: export PATH=$PATH:~/.agents/skills/csv_migrate_util")
	},
}

// extractSkillFiles рекурсивно копирует содержимое embed-папки скилла
// в целевую директорию. В embed FS пути всегда используют '/', поэтому
// для них применяется path.*, а для целевой ФС — filepath.*.
func extractSkillFiles(skillName, targetDir string) error {
	// WalkDir работает с embed без завершающих слэшей (ReadDir("csv_migrate_util/") падает)
	return fs.WalkDir(skills.SkillFolder, skillName, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Корневую папку скилла пропускаем, создаётся через MkdirAll ниже
		if p == skillName {
			return os.MkdirAll(targetDir, 0755)
		}

		// Относительный путь внутри скилла (используется как есть в embed)
		rel := p[len(skillName)+1:]
		dst := filepath.Join(targetDir, filepath.FromSlash(rel))

		if d.IsDir() {
			return os.MkdirAll(dst, 0755)
		}

		data, err := skills.SkillFolder.ReadFile(p)
		if err != nil {
			return fmt.Errorf("ошибка чтения файла %s: %w", p, err)
		}

		// Скрипты делаем исполняемыми
		mode := fs.FileMode(0644)
		if path.Ext(p) == ".sh" {
			mode = 0755
		}

		if err := os.WriteFile(dst, data, mode); err != nil {
			return fmt.Errorf("ошибка записи файла %s: %w", dst, err)
		}
		return nil
	})
}

func init() {
	rootCmd.AddCommand(installSkillCmd)
}
