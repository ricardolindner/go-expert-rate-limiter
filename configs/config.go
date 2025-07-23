package config

import (
	"log"
	"os"
	"strconv"
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
		log.Printf("Não foi possível carregar o arquivo .env, usando variáveis de ambiente: %v", err)
	}

	cfg := &Config{
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            getEnvAsInt("REDIS_DB", 0),
		DefaultMaxRequests: getEnvAsInt("DEFAULT_MAX_REQUESTS", 10),
		DefaultBlockTime:   getEnvAsDuration("DEFAULT_BLOCK_TIME_MINUTES", 1),
		TokenLimits:        make(map[string]TokenLimitConfig),
	}

	for _, env := range os.Environ() {
		if len(env) > 6 && env[0:6] == "TOKEN_" {
			parts := splitEnvVar(env)
			if len(parts) >= 3 && parts[len(parts)-1] == "MAX" {
				token := parts[1]
				for i := 2; i < len(parts)-1; i++ {
					token += "_" + parts[i]
				}

				maxReqsStr := os.Getenv("TOKEN_" + token + "_MAX")
				blockTimeStr := os.Getenv("TOKEN_" + token + "_BLOCK_MINUTES")

				maxReqs, err := strconv.Atoi(maxReqsStr)
				if err != nil {
					log.Printf("Erro ao parsear limite de requisições para token %s: %v", token, err)
					continue
				}

				blockMinutes, err := strconv.Atoi(blockTimeStr)
				if err != nil {
					log.Printf("Erro ao parsear tempo de bloqueio para token %s: %v", token, err)
					continue
				}
				cfg.TokenLimits[token] = TokenLimitConfig{
					MaxRequests: maxReqs,
					BlockTime:   time.Duration(blockMinutes) * time.Minute,
				}
				log.Printf("Configurado limite para token '%s': %d req/s, bloqueio de %s", token, maxReqs, time.Duration(blockMinutes)*time.Minute)
			}
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

func splitEnvVar(envVar string) []string {
	var parts []string
	var currentPart string
	for i, r := range envVar {
		if r == '=' {
			if currentPart != "" {
				parts = append(parts, currentPart)
			}
			parts = append(parts, envVar[i+1:])
			return parts
		}
		if r == '_' {
			parts = append(parts, currentPart)
			currentPart = ""
		} else {
			currentPart += string(r)
		}
	}
	if currentPart != "" {
		parts = append(parts, currentPart)
	}
	return parts
}
