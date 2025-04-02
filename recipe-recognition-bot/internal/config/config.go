package config

import (
	"github.com/spf13/viper"
)

// Config содержит все конфигурационные параметры приложения
type Config struct {
	TelegramToken     string `mapstructure:"TELEGRAM_TOKEN"`
	OpenAIAPIKey      string `mapstructure:"OPENAI_API_KEY"`
	PostgresURI       string `mapstructure:"POSTGRES_URI"`
	LogLevel          string `mapstructure:"LOG_LEVEL"`
	AppEnvironment    string `mapstructure:"APP_ENVIRONMENT"`
	MaxRecipesPerUser int    `mapstructure:"MAX_RECIPES_PER_USER"`
}

// LoadConfig загружает конфигурацию из файла и переменных окружения
func LoadConfig(path string) (*Config, error) {
	var config Config

	viper.AddConfigPath(path)
	viper.SetConfigName("app")
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	// Установка значений по умолчанию
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}

	if config.AppEnvironment == "" {
		config.AppEnvironment = "development"
	}

	if config.MaxRecipesPerUser == 0 {
		config.MaxRecipesPerUser = 50
	}

	return &config, nil
}
