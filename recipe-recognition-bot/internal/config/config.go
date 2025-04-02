package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"strconv"
)

// Config содержит все конфигурационные параметры приложения
type Config struct {
	TelegramToken     string
	OpenAIAPIKey      string
	PostgresURI       string
	LogLevel          string
	AppEnvironment    string
	MaxRecipesPerUser int
}

// LoadConfig загружает конфигурацию из переменных окружения
func LoadConfig() (*Config, error) {
	// Загружаем переменные из файла app.env
	err := godotenv.Load("app.env")
	if err != nil {
		return nil, fmt.Errorf("error loading app.env file: %w", err)
	}

	maxRecipes := 50
	if maxStr := os.Getenv("MAX_RECIPES_PER_USER"); maxStr != "" {
		if max, err := strconv.Atoi(maxStr); err == nil && max > 0 {
			maxRecipes = max
		}
	}

	return &Config{
		TelegramToken:     os.Getenv("TELEGRAM_TOKEN"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		PostgresURI:       os.Getenv("POSTGRES_URI"),
		LogLevel:          getEnvOrDefault("LOG_LEVEL", "info"),
		AppEnvironment:    getEnvOrDefault("APP_ENVIRONMENT", "development"),
		MaxRecipesPerUser: maxRecipes,
	}, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
