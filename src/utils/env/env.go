package env

import (
	"errors"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("failed to load environment variales from .env file. error: %v\n", err)
	}
}

func GetIntEnvWithDefault(key string, df int) int {
	env := os.Getenv(key)
	if env == "" {
		return df
	}

	val, err := strconv.Atoi(env)
	if err != nil {
		log.Fatalf("failed to parse int from env. value: %v. error: %v\n", env, err)
	}

	return val
}

func GetDurationEnvWithDefault(key string, df time.Duration) time.Duration {
	env := os.Getenv(key)
	if env == "" {
		return df
	}

	duration, err := time.ParseDuration(env)
	if err != nil {
		log.Fatalf("failed to parse time duration from env. value: %v. error: %v\n", env, err)
	}

	return duration
}

func GetMappedEnvWithDefault[V any](key string, df V, m map[string]V) V {
	env := os.Getenv(key)
	if env == "" {
		return df
	}

	val, ok := m[env]
	if !ok {
		log.Fatalf("failed to map variable from env. value: %v", env)
	}

	return val
}

func GetEnvWithDefault(key string, df string) string {
	if env, ok := os.LookupEnv(key); !ok {
		return df
	} else {
		return env
	}
}
