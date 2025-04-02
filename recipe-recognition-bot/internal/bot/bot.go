package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/TelegramBot/recipe-recognition-bot/internal/database"
	"github.com/TelegramBot/recipe-recognition-bot/internal/database/generated"
	"github.com/TelegramBot/recipe-recognition-bot/internal/recipes"
	"github.com/TelegramBot/recipe-recognition-bot/internal/vision"
)

// CommandHandler обрабатывает команды бота
type CommandHandler func(ctx context.Context, update tgbotapi.Update) error

// Bot представляет телеграм-бота
type Bot struct {
	api             *tgbotapi.BotAPI
	logger          *zap.Logger
	dbManager       *database.DBManager
	visionService   *vision.OpenAIVision
	recipeGenerator *recipes.RecipeGenerator
	commands        map[string]CommandHandler
	maxRecipes      int
}

// NewBot создает новый экземпляр бота
func NewBot(token string, logger *zap.Logger, dbManager *database.DBManager,
	visionService *vision.OpenAIVision, recipeGenerator *recipes.RecipeGenerator,
	maxRecipes int) (*Bot, error) {

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create BotAPI: %w", err)
	}

	b := &Bot{
		api:             bot,
		logger:          logger,
		dbManager:       dbManager,
		visionService:   visionService,
		recipeGenerator: recipeGenerator,
		commands:        make(map[string]CommandHandler),
		maxRecipes:      maxRecipes,
	}

	// Регистрируем обработчики команд
	b.commands["/start"] = b.handleStartCommand
	b.commands["/help"] = b.handleHelpCommand
	b.commands["/recipes"] = b.handleRecipesCommand

	return b, nil
}

// Start запускает бота
func (b *Bot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// Устанавливаем команды бота
	_, err := b.api.Request(tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "start", Description: "Начать работу с ботом"},
		tgbotapi.BotCommand{Command: "help", Description: "Получить справку"},
		tgbotapi.BotCommand{Command: "recipes", Description: "Просмотреть сохраненные рецепты"},
	))
	if err != nil {
		return fmt.Errorf("failed to set bot commands: %w", err)
	}

	updates := b.api.GetUpdatesChan(u)

	b.logger.Info("Bot started and listening for updates")

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("Stopping bot due to context cancellation")
			return nil
		case update := <-updates:
			go func(update tgbotapi.Update) {
				if err := b.handleUpdate(ctx, update); err != nil {
					b.logger.Error("Failed to handle update",
						zap.Int("update_id", update.UpdateID),
						zap.Error(err),
					)
				}
			}(update)
		}
	}
}

// handleUpdate обрабатывает новые сообщения от пользователей
func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) error {
	// Логируем информацию о полученном обновлении
	if update.Message != nil {
		b.logger.Info("Received message",
			zap.Int64("chat_id", update.Message.Chat.ID),
			zap.String("username", update.Message.From.UserName),
			zap.String("text", update.Message.Text),
		)
	}

	// Обрабатываем команды
	if update.Message != nil && update.Message.IsCommand() {
		cmd := update.Message.Command()
		if handler, ok := b.commands["/"+cmd]; ok {
			return handler(ctx, update)
		}
		return b.handleUnknownCommand(ctx, update)
	}

	// Обрабатываем фотографии
	if update.Message != nil && update.Message.Photo != nil {
		return b.handlePhotoMessage(ctx, update)
	}

	// Обрабатываем callback-запросы (нажатия на инлайн-кнопки)
	if update.CallbackQuery != nil {
		return b.handleCallbackQuery(ctx, update)
	}

	// Просто отвечаем на все остальные текстовые сообщения
	if update.Message != nil && update.Message.Text != "" {
		// Проверяем текст сообщения для кнопок
		switch update.Message.Text {
		case "Помощь":
			return b.handleHelpCommand(ctx, update)
		case "Мои рецепты":
			return b.handleRecipesCommand(ctx, update)
		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID,
				"Пожалуйста, отправьте фотографию продуктов или используйте команды /help для получения списка доступных команд.")
			_, err := b.api.Send(msg)
			return err
		}
	}

	return nil
}

// handleStartCommand обрабатывает команду /start
func (b *Bot) handleStartCommand(ctx context.Context, update tgbotapi.Update) error {
	user := update.Message.From
	chatID := update.Message.Chat.ID

	// Регистрируем пользователя в базе данных
	_, err := b.dbManager.GetUserOrCreate(ctx, user.ID, user.UserName, user.FirstName, user.LastName)
	if err != nil {
		b.logger.Error("Failed to register user",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)
		return err
	}

	// Отправляем приветственное сообщение
	welcomeText := fmt.Sprintf(
		"Здравствуйте, %s!\n\n"+
			"Я бот для распознавания продуктов и генерации рецептов.\n\n"+
			"Отправьте мне фотографию продуктов, и я предложу рецепт блюда, которое можно приготовить из них.\n\n"+
			"Доступные команды:\n"+
			"/help - получить справку\n"+
			"/recipes - просмотреть сохраненные рецепты",
		user.FirstName,
	)

	msg := tgbotapi.NewMessage(chatID, welcomeText)
	msg.ParseMode = tgbotapi.ModeMarkdown

	// Создаем клавиатуру для удобства
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Помощь"),
			tgbotapi.NewKeyboardButton("Мои рецепты"),
		),
	)

	_, err = b.api.Send(msg)
	return err
}

// handleHelpCommand обрабатывает команду /help
func (b *Bot) handleHelpCommand(ctx context.Context, update tgbotapi.Update) error {
	helpText := `*Как пользоваться ботом:*

1. Отправьте фотографию продуктов
2. Бот распознает продукты на фото
3. Бот предложит рецепт на основе распознанных продуктов
4. Вы можете сохранить рецепт для будущего использования

*Команды:*
/start - начать работу с ботом
/help - получить эту справку
/recipes - просмотреть сохраненные рецепты`

	var msg tgbotapi.MessageConfig
	if update.CallbackQuery != nil {
		msg = tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, helpText)
		// Отвечаем на callback запрос, чтобы убрать загрузку с кнопки
		callbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
		b.api.Request(callbackCfg)
	} else {
		msg = tgbotapi.NewMessage(update.Message.Chat.ID, helpText)
	}

	msg.ParseMode = tgbotapi.ModeMarkdown
	_, err := b.api.Send(msg)
	return err
}

// handleUnknownCommand обрабатывает неизвестные команды
func (b *Bot) handleUnknownCommand(ctx context.Context, update tgbotapi.Update) error {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID,
		"Неизвестная команда. Используйте /help для получения списка доступных команд.")
	_, err := b.api.Send(msg)
	return err
}

// handleRecipesCommand обрабатывает команду /recipes
func (b *Bot) handleRecipesCommand(ctx context.Context, update tgbotapi.Update) error {
	var user *tgbotapi.User
	var chatID int64

	if update.CallbackQuery != nil {
		user = update.CallbackQuery.From
		chatID = update.CallbackQuery.Message.Chat.ID

		// Отвечаем на callback запрос, чтобы убрать загрузку с кнопки
		callbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
		b.api.Request(callbackCfg)
	} else {
		user = update.Message.From
		chatID = update.Message.Chat.ID
	}

	// Получаем пользователя из базы данных
	dbUser, err := b.dbManager.GetUserOrCreate(ctx, user.ID, user.UserName, user.FirstName, user.LastName)
	if err != nil {
		return err
	}

	// Получаем список рецептов
	recipes, err := b.dbManager.Queries.ListUserRecipes(ctx, generated.ListUserRecipesParams{
		UserID: dbUser.ID,
		Limit:  int32(b.maxRecipes),
	})

	if err != nil {
		return err
	}

	if len(recipes) == 0 {
		msg := tgbotapi.NewMessage(chatID, "У вас пока нет сохраненных рецептов. "+
			"Отправьте фотографию продуктов, чтобы получить рецепт.")
		_, err = b.api.Send(msg)
		return err
	}

	// Создаем инлайн клавиатуру с рецептами
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, recipe := range recipes {
		button := tgbotapi.NewInlineKeyboardButtonData(
			recipe.RecipeTitle,
			fmt.Sprintf("recipe:%d", recipe.ID),
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{button})
	}

	msg := tgbotapi.NewMessage(chatID, "Ваши сохраненные рецепты:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	_, err = b.api.Send(msg)
	return err
}

// handleCallbackQuery обрабатывает нажатия на инлайн-кнопки
func (b *Bot) handleCallbackQuery(ctx context.Context, update tgbotapi.Update) error {
	data := update.CallbackQuery.Data
	chatID := update.CallbackQuery.Message.Chat.ID
	user := update.CallbackQuery.From

	// Проверяем, что это запрос на просмотр рецепта
	if len(data) > 7 && data[0:7] == "recipe:" {
		// Извлекаем ID рецепта
		recipeIDStr := data[7:]
		recipeID, err := strconv.Atoi(recipeIDStr)
		if err != nil {
			return err
		}

		// Получаем пользователя из базы данных
		dbUser, err := b.dbManager.GetUserOrCreate(ctx, user.ID, user.UserName, user.FirstName, user.LastName)
		if err != nil {
			return err
		}

		// Получаем рецепт из базы данных
		recipe, err := b.dbManager.Queries.GetRecipe(ctx, generated.GetRecipeParams{
			ID:     int32(recipeID),
			UserID: dbUser.ID,
		})
		if err != nil {
			// Отвечаем на callback запрос
			callbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			b.api.Request(callbackCfg)

			errorMsg := tgbotapi.NewMessage(chatID, "Не удалось найти рецепт. Возможно, он был удален.")
			_, err = b.api.Send(errorMsg)
			return err
		}

		// Отвечаем на callback запрос
		callbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
		b.api.Request(callbackCfg)

		// Отправляем рецепт пользователю
		recipeMsg := tgbotapi.NewMessage(chatID, recipe.RecipeContent)
		recipeMsg.ParseMode = tgbotapi.ModeMarkdown

		// Добавляем кнопку для удаления рецепта
		recipeMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить рецепт", fmt.Sprintf("delete:%d", recipe.ID)),
				tgbotapi.NewInlineKeyboardButtonData("« Назад к списку", "list_recipes"),
			),
		)

		_, err = b.api.Send(recipeMsg)
		return err
	} else if len(data) > 7 && data[0:7] == "delete:" {
		// Запрос на удаление рецепта
		recipeIDStr := data[7:]
		recipeID, err := strconv.Atoi(recipeIDStr)
		if err != nil {
			return err
		}

		// Получаем пользователя из базы данных
		dbUser, err := b.dbManager.GetUserOrCreate(ctx, user.ID, user.UserName, user.FirstName, user.LastName)
		if err != nil {
			return err
		}

		// Удаляем рецепт
		err = b.dbManager.Queries.DeleteRecipe(ctx, generated.DeleteRecipeParams{
			ID:     int32(recipeID),
			UserID: dbUser.ID,
		})
		if err != nil {
			callbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			b.api.Request(callbackCfg)

			errorMsg := tgbotapi.NewMessage(chatID, "Не удалось удалить рецепт.")
			_, err = b.api.Send(errorMsg)
			return err
		}

		// Отвечаем на callback запрос
		callbackCfg := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
		b.api.Request(callbackCfg)

		// Удаляем сообщение с рецептом
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, update.CallbackQuery.Message.MessageID)
		_, _ = b.api.Request(deleteMsg)

		// Отправляем подтверждение
		confirmMsg := tgbotapi.NewMessage(chatID, "Рецепт удален. Используйте /recipes для просмотра оставшихся рецептов.")
		_, err = b.api.Send(confirmMsg)
		return err
	} else if data == "list_recipes" {
		// Запрос на возврат к списку рецептов
		return b.handleRecipesCommand(ctx, update)
	}

	return nil
}

// handlePhotoMessage обрабатывает сообщения с фотографиями
func (b *Bot) handlePhotoMessage(ctx context.Context, update tgbotapi.Update) error {
	user := update.Message.From
	chatID := update.Message.Chat.ID

	// Регистрируем пользователя в базе данных
	dbUser, err := b.dbManager.GetUserOrCreate(ctx, user.ID, user.UserName, user.FirstName, user.LastName)
	if err != nil {
		return err
	}

	// Отправляем сообщение о начале обработки
	processingMsg := tgbotapi.NewMessage(chatID, "Обрабатываю фотографию... Это может занять несколько секунд.")
	sentMsg, err := b.api.Send(processingMsg)
	if err != nil {
		return err
	}

	// Получаем файл с максимальным размером
	photos := *update.Message.Photo
	fileID := photos[len(photos)-1].FileID

	// Загружаем фото из Telegram
	fileURL, err := b.api.GetFileDirectURL(fileID)
	if err != nil {
		errMsg := tgbotapi.NewMessage(chatID, "Не удалось загрузить фотографию. Пожалуйста, попробуйте снова.")
		_, _ = b.api.Send(errMsg)
		return err
	}

	// Загружаем изображение
	photoResp, err := http.Get(fileURL)
	if err != nil {
		errMsg := tgbotapi.NewMessage(chatID, "Ошибка при загрузке изображения. Пожалуйста, попробуйте снова.")
		_, _ = b.api.Send(errMsg)
		return err
	}
	defer photoResp.Body.Close()

	photoData, err := io.ReadAll(photoResp.Body)
	if err != nil {
		errMsg := tgbotapi.NewMessage(chatID, "Ошибка при чтении изображения. Пожалуйста, попробуйте снова.")
		_, _ = b.api.Send(errMsg)
		return err
	}

	// Распознаем продукты на фото
	recognizedItems, err := b.visionService.RecognizeProductsFromImage(ctx, bytes.NewReader(photoData))
	if err != nil {
		errMsg := tgbotapi.NewMessage(chatID, "Не удалось распознать продукты на фотографии. Пожалуйста, попробуйте сделать более четкий снимок.")
		_, _ = b.api.Send(errMsg)
		return err
	}

	// Отправляем список распознанных продуктов
	itemsList := ""
	for i, item := range recognizedItems.Items {
		itemsList += fmt.Sprintf("%d. %s\n", i+1, item)
	}

	recognizedMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Распознанные продукты:\n%s\n\nГенерирую рецепт...", itemsList))
	_, err = b.api.Send(recognizedMsg)
	if err != nil {
		return err
	}

	// Генерируем рецепт
	recipe, err := b.recipeGenerator.GenerateRecipe(ctx, recognizedItems.Items)
	if err != nil {
		errMsg := tgbotapi.NewMessage(chatID, "Не удалось сгенерировать рецепт. Пожалуйста, попробуйте снова.")
		_, _ = b.api.Send(errMsg)
		return err
	}

	// Форматируем рецепт для отображения
	formattedRecipe := b.recipeGenerator.FormatRecipe(recipe)

	// Сохраняем рецепт в базе данных
	ingredientsJSON, _ := json.Marshal(recipe.Ingredients)
	_, err = b.dbManager.Queries.SaveRecipe(ctx, generated.SaveRecipeParams{
		UserID:        dbUser.ID,
		RecipeTitle:   recipe.Title,
		RecipeContent: formattedRecipe,
		Ingredients:   ingredientsJSON,
	})
	if err != nil {
		b.logger.Error("Failed to save recipe",
			zap.Error(err),
		)
	}

	// Отправляем рецепт пользователю
	recipeMsg := tgbotapi.NewMessage(chatID, formattedRecipe)
	recipeMsg.ParseMode = tgbotapi.ModeMarkdown

	// Удаляем сообщение о процессе обработки
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, sentMsg.MessageID)
	_, _ = b.api.Request(deleteMsg)

	_, err = b.api.Send(recipeMsg)
	return err
}
