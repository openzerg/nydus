package config

import (
	"os"
	"strconv"
)

type Config struct {
	Host         string
	Port         int
	PublicURL    string
	DBPath       string
	JWTSecret    string
	CerebrateURL string
	AdminToken   string

	S3Endpoint  string
	S3Region    string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
}

func Load() *Config {
	return &Config{
		Host:         getEnv("NYDUS_HOST", "0.0.0.0"),
		Port:         getEnvInt("NYDUS_PORT", 15318),
		PublicURL:    getEnv("NYDUS_PUBLIC_URL", ""),
		DBPath:       getEnv("NYDUS_DB_PATH", "/var/lib/nydus/nydus.db"),
		JWTSecret:    getEnv("NYDUS_JWT_SECRET", "change-me-in-production"),
		CerebrateURL: getEnv("CEREBRATE_URL", ""),
		AdminToken:   getEnv("CEREBRATE_ADMIN_TOKEN", ""),

		S3Endpoint:  getEnv("S3_ENDPOINT", ""),
		S3Region:    getEnv("S3_REGION", "garage"),
		S3Bucket:    getEnv("S3_BUCKET", ""),
		S3AccessKey: getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey: getEnv("S3_SECRET_KEY", ""),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
