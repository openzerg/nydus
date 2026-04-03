package config

import (
	"os"
	"strconv"
)

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Cerebrate  CerebrateConfig
	Mutalisk   MutaliskConfig
	AdminToken string
}

type ServerConfig struct {
	Host string
	Port int
}

type DatabaseConfig struct {
	Path string
}

type CerebrateConfig struct {
	Host string
	Port int
}

type MutaliskConfig struct {
	// DefaultPort is used when Nydus derives the Mutalisk URL from instance IP.
	DefaultPort int
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: getEnv("NYDUS_HOST", "0.0.0.0"),
			Port: getEnvInt("NYDUS_PORT", 15318),
		},
		Database: DatabaseConfig{
			Path: getEnv("NYDUS_DB_PATH", "/var/lib/nydus/nydus.db"),
		},
		Cerebrate: CerebrateConfig{
			Host: getEnv("NYDUS_CEREBRATE_HOST", "10.0.0.1"),
			Port: getEnvInt("NYDUS_CEREBRATE_PORT", 15319),
		},
		Mutalisk: MutaliskConfig{
			DefaultPort: getEnvInt("NYDUS_MUTALISK_PORT", 15317),
		},
		AdminToken: os.Getenv("NYDUS_ADMIN_TOKEN"),
	}
}
