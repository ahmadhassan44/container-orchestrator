package config

import (
	"os"
	"strconv"
)

type Config struct {
	// CPU threshold percentage (0-100) beyond which we don't schedule on a worker
	MaxCPUThreshold float64

	// Pre-spawn threshold: spawn new container when all workers are above this %
	PreSpawnThreshold float64

	// Gateway HTTP port
	GatewayPort int

	// Worker base port (8001, 8002, 8003 for cores 1, 2, 3)
	WorkerBasePort int

	// Initial workers to spawn on startup
	InitialWorkers int
}

// LoadConfig reads configuration from environment variables with sensible defaults
func LoadConfig() *Config {
	return &Config{
		MaxCPUThreshold:   getEnvAsFloat("MAX_CPU_THRESHOLD", 80.0),
		PreSpawnThreshold: getEnvAsFloat("PRESPAWN_THRESHOLD", 70.0),
		GatewayPort:       getEnvAsInt("GATEWAY_PORT", 3000),
		WorkerBasePort:    getEnvAsInt("WORKER_BASE_PORT", 8000),
		InitialWorkers:    getEnvAsInt("INITIAL_WORKERS", 1),
	}
}

func getEnvAsFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return defaultVal
}
