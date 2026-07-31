package config

import (
	"os"
)

type Config struct {
	Port      string
	TargetAPI string
	StoreType string // "memory" or "redis"
	RedisAddr string
}

func LoadConfig() *Config {
	return &Config{
		Port:      getEnvOrDefault("PORT", ":8080"),
		TargetAPI: getEnvOrDefault("TARGET_API", "https://api.openai.com"),
		StoreType: getEnvOrDefault("STORE_TYPE", "memory"),
		RedisAddr: getEnvOrDefault("REDIS_ADDR", "localhost:6379"),
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultVal
}
