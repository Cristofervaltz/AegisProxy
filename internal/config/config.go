package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           string
	TargetAPI      string
	StoreType      string // "memory" or "redis"
	RedisAddr      string
	RateLimitRPS   float64
	RateLimitBurst int
}

func LoadConfig() *Config {
	return &Config{
		Port:           getEnvOrDefault("PORT", ":8080"),
		TargetAPI:      getEnvOrDefault("TARGET_API", "https://api.openai.com"),
		StoreType:      getEnvOrDefault("STORE_TYPE", "memory"),
		RedisAddr:      getEnvOrDefault("REDIS_ADDR", "localhost:6379"),
		RateLimitRPS:   getEnvFloatOrDefault("RATE_LIMIT_RPS", 10.0), // 10 requests per second default
		RateLimitBurst: getEnvIntOrDefault("RATE_LIMIT_BURST", 20),   // burst size of 20
	}
}

func getEnvOrDefault(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		return val
	}
	return defaultVal
}

func getEnvFloatOrDefault(key string, defaultVal float64) float64 {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getEnvIntOrDefault(key string, defaultVal int) int {
	if val, exists := os.LookupEnv(key); exists && val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return defaultVal
}
