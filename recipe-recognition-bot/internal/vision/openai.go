package vision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// OpenAIVision обрабатывает запросы к OpenAI API для распознавания продуктов
type OpenAIVision struct {
	client *openai.Client
	logger *zap.Logger
}

// RecognizedItems содержит список распознанных продуктов
type RecognizedItems struct {
	Items []string `json:"items"`
}

// NewOpenAIVision создает новый экземпляр OpenAIVision
func NewOpenAIVision(apiKey string, logger *zap.Logger) *OpenAIVision {
	client := openai.NewClient(apiKey)
	return &OpenAIVision{
		client: client,
		logger: logger,
	}
}

// RecognizeProductsFromImage распознает продукты на изображении
func (o *OpenAIVision) RecognizeProductsFromImage(ctx context.Context, imageData io.Reader) (*RecognizedItems, error) {
	// Конвертируем изображение в base64
	data, err := io.ReadAll(imageData)
	if err != nil {
		return nil, fmt.Errorf("error reading image data: %w", err)
	}

	base64Image := base64.StdEncoding.EncodeToString(data)

	// Создаем запрос к OpenAI API с инструкцией распознать продукты
	resp, err := o.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4VisionPreview,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleUser,
					MultiContent: []openai.ChatMessagePart{
						{
							Type: openai.ChatMessagePartTypeText,
							Text: "Определи все продукты питания на этом изображении. Перечисли только названия продуктов в формате JSON массива с ключом 'items'. Не добавляй никаких объяснений.",
						},
						{
							Type: openai.ChatMessagePartTypeImageURL,
							ImageURL: &openai.ChatMessageImageURL{
								URL:    fmt.Sprintf("data:image/jpeg;base64,%s", base64Image),
								Detail: openai.ImageURLDetailHigh,
							},
						},
					},
				},
			},
			MaxTokens: 300,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("error calling OpenAI API: %w", err)
	}

	// Извлекаем JSON из ответа
	content := resp.Choices[0].Message.Content
	o.logger.Debug("OpenAI response", zap.String("content", content))

	// Пробуем найти JSON в тексте
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no valid JSON found in the response: %s", content)
	}

	jsonContent := content[jsonStart : jsonEnd+1]

	var recognized RecognizedItems
	if err := json.Unmarshal([]byte(jsonContent), &recognized); err != nil {
		return nil, fmt.Errorf("error parsing JSON from response: %w", err)
	}

	// Если нет элементов в списке, считаем что нет распознанных продуктов
	if len(recognized.Items) == 0 {
		return nil, fmt.Errorf("no food items recognized in the image")
	}

	return &recognized, nil
}
