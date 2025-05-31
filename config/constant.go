package config

import (
	"os"
	"path/filepath"
	"time"
)

const (
	Version              = "25.0517"
	CatalogVersionInfo   = "#版本=1.0" // 书签目录版本TXT
	defaultUserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
	defaultFileExtension = ".jpg"

	defaultRetry   = 3
	defaultTimeout = 300 * time.Second
	defaultQuality = 80
	defaultFormat  = "full/full/0/default.jpg"
)

func UserHomeDir() string {
	if os.PathSeparator == '\\' {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func BookgetHomeDir() string {
	home, err := os.UserHomeDir()
	if err == nil {
		// Unix-like: ~/bookget/path
		// Windows: ~\bookget\path
		configDir := filepath.Join(home, "bookget")
		if os.PathSeparator == '\\' { // Windows
			configDir = filepath.Join(home, "bookget")
		}
		homeDir := filepath.Join(configDir)
		if err := os.Mkdir(homeDir, 0755); err != nil && !os.IsExist(err) {
			return ""
		}
		return homeDir
	}
	return home
}

func CacheDir() string {
	return filepath.Join(BookgetHomeDir(), "cache")
}
