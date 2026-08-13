package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                    string
	DBPath                  string // DB_PATH — only used if internal/db is wired in
	AutosaveIntervalSeconds int
	PyroscopeServerAddress  string
}

func Load() *Config {
	return &Config{
		Port:                    getEnv("PORT", "8080"),
		DBPath:                  getEnv("DB_PATH", "/data/palimpsest.db"),
		AutosaveIntervalSeconds: getEnvInt("AUTOSAVE_INTERVAL_SECONDS", 10),
		PyroscopeServerAddress:  getEnv("PYROSCOPE_SERVER_ADDRESS", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
