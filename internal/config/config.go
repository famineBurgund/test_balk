package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Port              string
	BaseSOAPServer    string
	JSONSaveDir       string
	InvoiceStorageDir string
	DislocSourceDir   string
	LogDir            string
	StateDir          string
	DBHost            string
	DBPort            string
	DBUser            string
	DBPassword        string
	DBName            string
}

func Load() (Config, error) {
	cfg := Config{
		Port:              getEnv("PORT", "50070"),
		BaseSOAPServer:    getEnv("BASE_SOAP_SERVER", "http://212.19.9.201:50049"),
		JSONSaveDir:       getEnv("JSON_SAVE_DIR", "/jsontoesat"),
		InvoiceStorageDir: getEnv("INVOICE_STORAGE_DIR", "/invoice_storage"),
		DislocSourceDir:   getEnv("DISLOC_SOURCE_DIR", "/jsontoesatm"),
		LogDir:            getEnv("LOG_DIR", "./logs"),
		StateDir:          getEnv("STATE_DIR", "./state"),
		DBHost:            os.Getenv("DB_HOST"),
		DBPort:            getEnv("DB_PORT", "5432"),
		DBUser:            os.Getenv("DB_USER"),
		DBPassword:        os.Getenv("DB_PASSWORD"),
		DBName:            os.Getenv("DB_NAME"),
	}

	if cfg.DBHost == "" || cfg.DBUser == "" || cfg.DBPassword == "" || cfg.DBName == "" {
		return Config{}, errors.New("DB_HOST, DB_USER, DB_PASSWORD and DB_NAME are required")
	}

	for _, dir := range []string{cfg.JSONSaveDir, cfg.InvoiceStorageDir, cfg.DislocSourceDir, cfg.LogDir, cfg.StateDir} {
		if err := os.MkdirAll(filepath.Clean(dir), 0o755); err != nil {
			return Config{}, fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	return cfg, nil
}

func (c Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		c.DBUser,
		c.DBPassword,
		c.DBHost,
		c.DBPort,
		c.DBName,
	)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
