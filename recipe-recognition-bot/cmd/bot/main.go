package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/TelegramBot/recipe-recognition-bot/internal/bot"
	"github.com/TelegramBot/recipe-recognition-bot/internal/config"
	"github.com/TelegramBot/recipe-recognition-bot/internal/database"
	"github.com/TelegramBot/recipe-recognition-bot/internal/recipes"
	"github.com/TelegramBot/recipe-recognition-bot/internal/vision"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Настраиваем логгер
	var logger *zap.Logger
	if cfg.AppEnvironment == "development" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting the Recipe Recognition Bot")

	// Создаем контекст с возможностью отмены
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Инициализируем соединение с базой данных
	dbManager, err := database.NewDBManager(ctx, cfg.PostgresURI, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database connection", zap.Error(err))
	}
	defer dbManager.Close()

	// Запускаем миграции
	if err := dbManager.RunMigrations("migrations"); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}

	// Инициализируем сервис распознавания продуктов
	visionService := vision.NewOpenAIVision(cfg.OpenAIAPIKey, logger)

	// Инициализируем генератор рецептов
	recipeGenerator := recipes.NewRecipeGenerator(cfg.OpenAIAPIKey, logger)

	// Создаем бота
	bot, err := bot.NewBot(
		cfg.TelegramToken,
		logger,
		dbManager,
		visionService,
		recipeGenerator,
		cfg.MaxRecipesPerUser,
	)
	if err != nil {
		logger.Fatal("Failed to create bot", zap.Error(err))
	}

	// Обрабатываем сигналы для изящного завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Запускаем бота в отдельной горутине
	go func() {
		if err := bot.Start(ctx); err != nil {
			logger.Error("Bot stopped with error", zap.Error(err))
			cancel()
		}
	}()

	// Ожидаем сигнала для завершения
	sig := <-sigChan
	logger.Info("Received termination signal", zap.String("signal", sig.String()))

	// Отменяем контекст и даем горутинам время на завершение
	cancel()
	time.Sleep(3 * time.Second)

	logger.Info("Bot stopped successfully")
}
