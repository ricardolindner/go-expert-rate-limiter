package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	DefaultMaxRequests int
	DefaultBlockTime   time.Duration
	TokenLimits        map[string]TokenLimitConfig
}

type TokenLimitConfig struct {
	MaxRequests int
	BlockTime   time.Duration
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Printf("ERROR: Not able to load .env file: %v", err)
	} else {
		log.Println("File .env loaded successfully!")
	}

	cfg := &Config{
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            getEnvAsInt("REDIS_DB", 0),
		DefaultMaxRequests: getEnvAsInt("DEFAULT_MAX_REQUESTS", 10),
		DefaultBlockTime:   getEnvAsDuration("DEFAULT_BLOCK_TIME_MINUTES", 1),
		TokenLimits:        make(map[string]TokenLimitConfig),
	}

	for _, envVar := range os.Environ() {
		pair := strings.SplitN(envVar, "=", 2)
		key := pair[0]

		if strings.HasPrefix(key, "TOKEN_") && strings.HasSuffix(key, "_MAX") {
			tokenName := strings.TrimPrefix(key, "TOKEN_")
			tokenName = strings.TrimSuffix(tokenName, "_MAX")

			maxRequestsStr := os.Getenv(key)
			blockTimeMinutesStr := os.Getenv("TOKEN_" + tokenName + "_BLOCK_MINUTES")

			maxRequests, err := strconv.Atoi(maxRequestsStr)
			if err != nil {
				log.Printf("WARN: Invalid value for %s: %s. Using 0.", key, maxRequestsStr)
				maxRequests = 0
			}

			blockTimeMinutes, err := strconv.Atoi(blockTimeMinutesStr)
			if err != nil {
				log.Printf("WARN: Invalid value for TOKEN_%s_BLOCK_MINUTES: %s. Using 0.", tokenName, blockTimeMinutesStr)
				blockTimeMinutes = 0
			}

			cfg.TokenLimits[tokenName] = TokenLimitConfig{
				MaxRequests: maxRequests,
				BlockTime:   time.Duration(blockTimeMinutes) * time.Minute,
			}
			log.Printf(
				"Loading limit for token '%s': %d requests, %d minutes block.",
				tokenName,
				maxRequests,
				int(cfg.TokenLimits[tokenName].BlockTime/time.Minute),
			)
		}
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	strValue := getEnv(key, "")
	if intValue, err := strconv.Atoi(strValue); err == nil {
		return intValue
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValueMinutes int) time.Duration {
	strValue := getEnv(key, "")
	if intValue, err := strconv.Atoi(strValue); err == nil {
		return time.Duration(intValue) * time.Minute
	}
	return time.Duration(defaultValueMinutes) * time.Minute
}
