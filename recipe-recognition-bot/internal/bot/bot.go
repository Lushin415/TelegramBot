package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"github.com/TelegramBot/recipe-recognition-bot/internal/database"
	dbmodels "github.com/TelegramBot/recipe-recognition-bot/internal/database/generated"
	"github.com/TelegramBot/recipe-recognition-bot/internal/recipes"
	"github.com/TelegramBot/recipe-recognition-bot/internal/vision"
)

// Bot представляет телеграм-бота
type Bot struct {
	api             *tgbotapi.BotAPI
	logger          *zap.Logger
	dbManager       *database.DBManager
	visionService   *vision.OpenAIVision
	recipeGenerator *recipes.RecipeGenerator
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

	return &Bot{
		api:             bot,
		logger:          logger,
		dbManager:       dbManager,
		visionService:   visionService,
		recipeGenerator: recipeGenerator,
		maxRecipes:      maxRecipes,
	}, nil
}

// Start запускает бота
func (b *Bot) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// Устанавливаем команды бота
	b.api.Request(tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "start", Description: "Начать работу с ботом"},
		tgbotapi.BotCommand{Command: "help", Description: "Получить справку"},
		tgbotapi.BotCommand{Command: "recipes", Description: "Просмотреть сохраненные рецепты"},
	))

	updates := b.api.GetUpdatesChan(u)

	b.logger.Info("Bot started")

	for {
		select {
		case <-ctx.Done():
			return nil
		case update := <-updates:
			go b.handleUpdate(ctx, update)
		}
	}
}

// handleUpdate обрабатывает новые сообщения
func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	// Обработка команд
	if update.Message != nil && update.Message.IsCommand() {
		cmd := update.Message.Command()
		switch cmd {
		case "start":
			b.handleStartCommand(ctx, update)
		case "help":
			b.handleHelpCommand(ctx, update)
		case "recipes":
			b.handleRecipesCommand(ctx, update)
		default:
			b.handleUnknownCommand(ctx, update)
		}
		return
	}

	// Обработка фотографий
	if update.Message != nil && update.Message.Photo != nil {
		b.handlePhotoMessage(ctx, update)
		return
	}

	// Обработка callback-запросов
	if update.CallbackQuery != nil {
		b.handleCallbackQuery(ctx, update)
		return
	}

	// Простые текстовые сообщения
	if update.Message != nil && update.Message.Text != "" {
		switch update.Message.Text {
		case "Помощь":
			b.handleHelpCommand(ctx, update)
		case "Мои рецепты":
			b.handleRecipesCommand(ctx, update)
		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID,
				"Отправьте фото продуктов или используйте команды (/help).")
			b.api.Send(msg)
		}
	}
}

// handleStartCommand обрабатывает команду /start
func (b *Bot) handleStartCommand(ctx context.Context, update tgbotapi.Update) {
	user := update.Message.From
	chatID := update.Message.Chat.ID

	// Регистрируем пользователя в БД
	b.dbManager.GetUserOrCreate(ctx, user.ID, user.UserName, user.FirstName, user.LastName)

	welcomeText := fmt.Sprintf(
		"Здравствуйте, %s!\n\n"+
			"Я бот для распознавания продуктов и генерации рецептов.\n\n"+
			"Отправьте мне фотографию продуктов, и я предложу рецепт.\n\n"+
			"Команды:\n"+
			"/help - справка\n"+
			"/recipes - сохраненные рецепты",
		user.FirstName,
	)

	msg := tgbotapi.NewMessage(chatID, welcomeText)
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Помощь"),
			tgbotapi.NewKeyboardButton("Мои рецепты"),
		),
	)

	b.api.Send(msg)
}

// handleHelpCommand обрабатывает команду /help
func (b *Bot) handleHelpCommand(ctx context.Context, update tgbotapi.Update) {
	helpText := `*Как пользоваться ботом:*

1. Отправьте фото продуктов
2. Бот распознает продукты 
3. Бот предложит рецепт
4. Вы можете сохранить рецепт

*Команды:*
/start - начать работу
/help - справка
/recipes - сохраненные рецепты`

	var msg tgbotapi.MessageConfig
	if update.CallbackQuery != nil {
		msg = tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, helpText)
		b.api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))
	} else {
		msg = tgbotapi.NewMessage(update.Message.Chat.ID, helpText)
	}

	msg.ParseMode = tgbotapi.ModeMarkdown
	b.api.Send(msg)
}

// handleUnknownCommand обрабатывает неизвестные команды
func (b *Bot) handleUnknownCommand(ctx context.Context, update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID,
		"Неизвестная команда. Используйте /help для списка команд.")
	b.api.Send(msg)
}

// handleRecipesCommand обрабатывает команду /recipes
func (b *Bot) handleRecipesCommand(ctx context.Context, update tgbotapi.Update) {
	var user *tgbotapi.User
	var chatID int64

	if update.CallbackQuery != nil {
		user = update.CallbackQuery.From
		chatID = update.CallbackQuery.Message.Chat.ID
		b.api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))
	} else {
		user = update.Message.From
		chatID = update.Message.Chat.ID
	}

	// Получаем пользователя из БД
	dbUser, _ := b.dbManager.GetUserOrCreate(ctx, user.ID, user.UserName, user.FirstName, user.LastName)

	// Получаем рецепты
	recipes, err := b.dbManager.Queries.ListUserRecipes(ctx, dbmodels.ListUserRecipesParams{
		UserID: dbUser.ID,
		Limit:  int32(b.maxRecipes),
	})

	if err != nil || len(recipes) == 0 {
		msg := tgbotapi.NewMessage(chatID, "У вас пока нет сохраненных рецептов. "+
			"Отправьте фото продуктов, чтобы получить рецепт.")
		b.api.Send(msg)
		return
	}

	// Создаем клавиатуру с рецептами
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
	b.api.Send(msg)
}

// handleCallbackQuery обрабатывает нажатия на инлайн-кнопки
func (b *Bot) handleCallbackQuery(ctx context.Context, update tgbotapi.Update) {
	data := update.CallbackQuery.Data
	chatID := update.CallbackQuery.Message.Chat.ID
	user := update.CallbackQuery.From

	// Просмотр рецепта
	if strings.HasPrefix(data, "recipe:") {
		recipeID, _ := strconv.Atoi(data[7:])
		dbUser, _ := b.dbManager.GetUserOrCreate(ctx, user.ID, user.UserName, user.FirstName, user.LastName)

		recipe, err := b.dbManager.Queries.GetRecipe(ctx, dbmodels.GetRecipeParams{
			ID:     int32(recipeID),
			UserID: dbUser.ID,
		})

		b.api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))

		if err != nil {
			errorMsg := tgbotapi.NewMessage(chatID, "Не удалось найти рецепт.")
			b.api.Send(errorMsg)
			return
		}

		recipeMsg := tgbotapi.NewMessage(chatID, recipe.RecipeContent)
		recipeMsg.ParseMode = tgbotapi.ModeMarkdown
		recipeMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", fmt.Sprintf("delete:%d", recipe.ID)),
				tgbotapi.NewInlineKeyboardButtonData("« Назад", "list_recipes"),
			),
		)

		b.api.Send(recipeMsg)
		return
	}

	// Удаление рецепта
	if strings.HasPrefix(data, "delete:") {
		recipeID, _ := strconv.Atoi(data[7:])
		dbUser, _ := b.dbManager.GetUserOrCreate(ctx, user.ID, user.UserName, user.FirstName, user.LastName)

		b.dbManager.Queries.DeleteRecipe(ctx, dbmodels.DeleteRecipeParams{
			ID:     int32(recipeID),
			UserID: dbUser.ID,
		})

		b.api.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))
		b.api.Request(tgbotapi.NewDeleteMessage(chatID, update.CallbackQuery.Message.MessageID))

		confirmMsg := tgbotapi.NewMessage(chatID, "Рецепт удален. Используйте /recipes для просмотра остальных.")
		b.api.Send(confirmMsg)
		return
	}

	// Возврат к списку
	if data == "list_recipes" {
		b.handleRecipesCommand(ctx, update)
	}
}

// handlePhotoMessage обрабатывает сообщения с фотографиями
func (b *Bot) handlePhotoMessage(ctx context.Context, update tgbotapi.Update) {
	user := update.Message.From
	chatID := update.Message.Chat.ID

	// Регистрируем пользователя
	dbUser, _ := b.dbManager.GetUserOrCreate(ctx, user.ID, user.UserName, user.FirstName, user.LastName)

	// Сообщение об обработке
	processingMsg := tgbotapi.NewMessage(chatID, "Обрабатываю фото... Это займет несколько секунд.")
	sentMsg, _ := b.api.Send(processingMsg)

	// Получаем фото
	photos := update.Message.Photo
	fileID := photos[len(photos)-1].FileID
	fileURL, err := b.api.GetFileDirectURL(fileID)

	if err != nil {
		errMsg := tgbotapi.NewMessage(chatID, "Не удалось загрузить фото. Попробуйте снова.")
		b.api.Send(errMsg)
		return
	}

	// Загружаем изображение
	photoResp, err := http.Get(fileURL)
	if err != nil {
		errMsg := tgbotapi.NewMessage(chatID, "Ошибка при загрузке изображения.")
		b.api.Send(errMsg)
		return
	}
	defer photoResp.Body.Close()

	photoData, _ := io.ReadAll(photoResp.Body)

	// Распознаем продукты
	recognizedItems, err := b.visionService.RecognizeProductsFromImage(ctx, bytes.NewReader(photoData))
	if err != nil {
		errMsg := tgbotapi.NewMessage(chatID, "Не удалось распознать продукты. Сделайте более четкий снимок.")
		b.api.Send(errMsg)
		return
	}

	// Отправляем список продуктов
	itemsList := ""
	for i, item := range recognizedItems.Items {
		itemsList += fmt.Sprintf("%d. %s\n", i+1, item)
	}

	recognizedMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Распознанные продукты:\n%s\n\nГенерирую рецепт...", itemsList))
	b.api.Send(recognizedMsg)

	// Генерируем рецепт
	recipe, err := b.recipeGenerator.GenerateRecipe(ctx, recognizedItems.Items)
	if err != nil {
		errMsg := tgbotapi.NewMessage(chatID, "Не удалось сгенерировать рецепт. Попробуйте снова.")
		b.api.Send(errMsg)
		return
	}

	// Форматируем и сохраняем рецепт
	formattedRecipe := b.recipeGenerator.FormatRecipe(recipe)
	ingredientsJSON, _ := json.Marshal(recipe.Ingredients)

	b.dbManager.Queries.SaveRecipe(ctx, dbmodels.SaveRecipeParams{
		UserID:        dbUser.ID,
		RecipeTitle:   recipe.Title,
		RecipeContent: formattedRecipe,
		Ingredients:   ingredientsJSON,
	})

	// Отправляем рецепт
	recipeMsg := tgbotapi.NewMessage(chatID, formattedRecipe)
	recipeMsg.ParseMode = tgbotapi.ModeMarkdown

	// Удаляем сообщение о процессе
	b.api.Request(tgbotapi.NewDeleteMessage(chatID, sentMsg.MessageID))
	b.api.Send(recipeMsg)
}
