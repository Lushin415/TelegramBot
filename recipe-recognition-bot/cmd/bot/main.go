package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/TelegramBot/recipe-recognition-bot/internal/bot"
	"github.com/TelegramBot/recipe-recognition-bot/internal/config"
	"github.com/TelegramBot/recipe-recognition-bot/internal/database"
	"github.com/TelegramBot/recipe-recognition-bot/internal/recipes"
	"github.com/TelegramBot/recipe-recognition-bot/internal/vision"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Настраиваем логгер
	var logger *zap.Logger
	if cfg.AppEnvironment == "development" {
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}
	defer logger.Sync()

	// Контекст с отменой для изящного завершения
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализация компонентов
	dbManager, err := database.NewDBManager(ctx, cfg.PostgresURI, logger)
	if err != nil {
		logger.Fatal("Database connection failed", zap.Error(err))
	}
	defer dbManager.Close()

	if err := dbManager.RunMigrations("migrations"); err != nil {
		logger.Fatal("Migration failed", zap.Error(err))
	}

	// Запуск бота
	b, err := bot.NewBot(
		cfg.TelegramToken,
		logger,
		dbManager,
		vision.NewOpenAIVision(cfg.OpenAIAPIKey, logger),
		recipes.NewRecipeGenerator(cfg.OpenAIAPIKey, logger),
		cfg.MaxRecipesPerUser,
	)
	if err != nil {
		logger.Fatal("Bot creation failed", zap.Error(err))
	}

	// Отслеживание сигналов для остановки
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Запуск бота в отдельной горутине
	go func() {
		if err := b.Start(ctx); err != nil {
			logger.Error("Bot error", zap.Error(err))
			cancel()
		}
	}()

	<-sigChan
	logger.Info("Shutting down...")
	cancel()
}
