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
	return &RecipeGenerator{
		client: openai.NewClient(apiKey),
		logger: logger,
	}
}

func (g *RecipeGenerator) GenerateRecipe(ctx context.Context, products []string) (*Recipe, error) {
	productsList := strings.Join(products, ", ")

	prompt := fmt.Sprintf(`На основе продуктов: %s
Придумай рецепт. Верни только JSON:
{
  "title": "Название рецепта",
  "ingredients": ["ингредиент 1", "ингредиент 2", ...],
  "instructions": "Инструкции"
}`, productsList)

	resp, err := g.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4,
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
		return nil, err
	}

	content := resp.Choices[0].Message.Content
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("некорректный ответ от API")
	}

	jsonContent := content[jsonStart : jsonEnd+1]
	var recipe Recipe
	if err := json.Unmarshal([]byte(jsonContent), &recipe); err != nil {
		return nil, err
	}

	if recipe.Title == "" || len(recipe.Ingredients) == 0 || recipe.Instructions == "" {
		return nil, fmt.Errorf("неполный рецепт от API")
	}

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
