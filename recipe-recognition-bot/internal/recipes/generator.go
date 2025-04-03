package recipes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

type RecipeGenerator struct {
	client *openai.Client
	logger *zap.Logger
}

type Recipe struct {
	Title        string   `json:"title"`
	Ingredients  []string `json:"ingredients"`
	Instructions string   `json:"instructions"`
}

func NewRecipeGenerator(apiKey string, logger *zap.Logger) *RecipeGenerator {
	// Создаем конфигурацию для OpenRouter вместо OpenAI
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://openrouter.ai/api/v1"
	// Проверка ключа
	if apiKey == "" {
		logger.Error("API ключ OpenRouter пустой")
		// Возможно выбросить ошибку здесь
	}
	return &RecipeGenerator{
		client: openai.NewClientWithConfig(config),
		logger: logger,
	}
}

func (g *RecipeGenerator) GenerateRecipe(ctx context.Context, products []string) (*Recipe, error) {
	productsList := strings.Join(products, ", ")
	g.logger.Info("Генерация рецепта", zap.Strings("продукты", products))

	prompt := fmt.Sprintf(`Вот список продуктов: %s

Ты - повар!
Задача: создать полный рецепт блюда, используя только эти продукты, рецепт должен быть в формате JSON.

Формат ответа - строго JSON (дается для примера):
{
  "title": "Название блюда",
  "ingredients": ["ингредиент 1 с количеством", "ингредиент 2 с количеством", "..."],
  "instructions": "Пошаговые инструкции по приготовлению"
}

Важно: верни ТОЛЬКО JSON без дополнительного текста!`, productsList)

	g.logger.Debug("Отправка запроса в OpenRouter", zap.String("prompt", prompt))

	resp, err := g.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: "deepseek/deepseek-chat:free",
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "Ты - эксперт кулинарии. Генерируешь рецепты из доступных продуктов.",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			MaxTokens: 1000,
		},
	)

	if err != nil {
		g.logger.Error("Ошибка запроса к OpenRouter", zap.Error(err))
		return nil, fmt.Errorf("ошибка API: %w", err)
	}

	content := resp.Choices[0].Message.Content
	g.logger.Debug("Ответ от OpenRouter", zap.String("content", content))

	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		g.logger.Error("Некорректный JSON в ответе",
			zap.Int("jsonStart", jsonStart),
			zap.Int("jsonEnd", jsonEnd),
			zap.String("content", content))
		return nil, fmt.Errorf("некорректный ответ от API: JSON не найден")
	}

	jsonContent := content[jsonStart : jsonEnd+1]
	g.logger.Debug("Извлеченный JSON", zap.String("json", jsonContent))

	var recipe Recipe
	if err := json.Unmarshal([]byte(jsonContent), &recipe); err != nil {
		g.logger.Error("Ошибка парсинга JSON", zap.Error(err), zap.String("json", jsonContent))
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	if recipe.Title == "" || len(recipe.Ingredients) == 0 || recipe.Instructions == "" {
		g.logger.Error("Неполный рецепт", zap.Any("recipe", recipe))
		return nil, fmt.Errorf("неполный рецепт от API: отсутствуют обязательные поля")
	}

	g.logger.Info("Рецепт успешно сгенерирован", zap.String("title", recipe.Title))
	return &recipe, nil
}

func (g *RecipeGenerator) FormatRecipe(recipe *Recipe) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("🍳 *%s*\n\n", recipe.Title))

	sb.WriteString("*Ингредиенты:*\n")
	for i, ingredient := range recipe.Ingredients {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, ingredient))
	}

	sb.WriteString("\n*Инструкции:*\n")
	sb.WriteString(recipe.Instructions)

	return sb.String()
}
