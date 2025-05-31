package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const configContent = `
`

// CreateConfigIfNotExists 检查并创建配置文件
func CreateConfigIfNotExists(configPath string) error {
	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		dir := filepath.Dir(configPath)
		if dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
		}

		// 文件不存在，创建并写入内容
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			return fmt.Errorf("创建配置文件失败: %w", err)
		}
		fmt.Printf("配置文件已创建: %s\n", configPath)
	} else if err != nil {
		// 其他错误
		return fmt.Errorf("检查配置文件失败: %w", err)
	} else {
		fmt.Printf("配置文件在: %s\n", configPath)
	}
	return nil
}
