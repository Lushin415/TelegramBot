package recipes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// RecipeGenerator генерирует рецепты на основе списка продуктов
type RecipeGenerator struct {
	client *openai.Client
	logger *zap.Logger
}

// Recipe содержит сгенерированный рецепт
type Recipe struct {
	Title        string   `json:"title"`
	Ingredients  []string `json:"ingredients"`
	Instructions string   `json:"instructions"`
}

// NewRecipeGenerator создает новый экземпляр RecipeGenerator
func NewRecipeGenerator(apiKey string, logger *zap.Logger) *RecipeGenerator {
	client := openai.NewClient(apiKey)
	return &RecipeGenerator{
		client: client,
		logger: logger,
	}
}

// GenerateRecipe генерирует рецепт на основе списка продуктов
func (g *RecipeGenerator) GenerateRecipe(ctx context.Context, products []string) (*Recipe, error) {
	// Формируем список продуктов
	productsList := strings.Join(products, ", ")

	// Создаем запрос к OpenAI API для генерации рецепта
	prompt := fmt.Sprintf(`На основе следующих продуктов, придумай рецепт.
Продукты: %s.

Учти, что можно использовать не все продукты из списка, и можно добавить базовые ингредиенты, которых нет в списке.
Верни рецепт в формате JSON со следующими полями:
{
  "title": "Название рецепта",
  "ingredients": ["ингредиент 1", "ингредиент 2", ...],
  "instructions": "Пошаговые инструкции по приготовлению"
}

Не добавляй никаких других полей или текста, только JSON.`, productsList)

	resp, err := g.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: "Ты - эксперт по кулинарии. Твоя задача - генерировать рецепты на основе доступных продуктов.",
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
		return nil, fmt.Errorf("error calling OpenAI API: %w", err)
	}

	// Извлекаем JSON из ответа
	content := resp.Choices[0].Message.Content
	g.logger.Debug("OpenAI response", zap.String("content", content))

	// Пробуем найти JSON в тексте
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no valid JSON found in the response: %s", content)
	}

	jsonContent := content[jsonStart : jsonEnd+1]

	var recipe Recipe
	if err := json.Unmarshal([]byte(jsonContent), &recipe); err != nil {
		return nil, fmt.Errorf("error parsing JSON from response: %w", err)
	}

	// Проверяем, что все нужные поля заполнены
	if recipe.Title == "" || len(recipe.Ingredients) == 0 || recipe.Instructions == "" {
		return nil, fmt.Errorf("invalid recipe data returned from API")
	}

	return &recipe, nil
}

// FormatRecipe форматирует рецепт для отображения пользователю
func (g *RecipeGenerator) FormatRecipe(recipe *Recipe) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("🍳 *%s*\n\n", recipe.Title))

	sb.WriteString("*Ингредиенты:*\n")
	for i, ingredient := range recipe.Ingredients {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, ingredient))
	}

	sb.WriteString("\n*Инструкции по приготовлению:*\n")
	sb.WriteString(recipe.Instructions)

	return sb.String()
}
