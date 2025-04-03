package vision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

type OpenAIVision struct {
	client *openai.Client
	logger *zap.Logger
}

type RecognizedItems struct {
	Items []string `json:"items"`
}

func NewOpenAIVision(apiKey string, logger *zap.Logger) *OpenAIVision {
	// Создаем конфигурацию для OpenRouter вместо OpenAI
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://openrouter.ai/api/v1"

	return &OpenAIVision{
		client: openai.NewClientWithConfig(config),
		logger: logger,
	}
}

func (o *OpenAIVision) RecognizeProductsFromImage(ctx context.Context, imageData io.Reader) (*RecognizedItems, error) {
	data, err := io.ReadAll(imageData)
	if err != nil {
		log.Println("Ошибка чтения изображения:", err)
		return nil, err
	}
	log.Println("Изображение прочитано, размер:", len(data))

	base64Image := base64.StdEncoding.EncodeToString(data)
	log.Println("Base64-кодирование выполнено, длина строки:", len(base64Image))

	log.Println("Отправляю запрос в OpenRouter с изображением...")

	// Создаем запрос с дополнительными параметрами для OpenRouter
	req := openai.ChatCompletionRequest{
		Model: "qwen/qwen-2.5-vl-7b-instruct:free", // Изменяем модель на Molmo
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: `List all food products in this image. 
Return only JSON: {"items": ["product1", "product2"]}.
Maximum 20 products.`,
						//Промт такой потому, что слишком много продуктов зацикливают нейросеть
					},
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: fmt.Sprintf("data:image/jpeg;base64,%s", base64Image),
						},
					},
				},
			},
		},
		MaxTokens: 300,
	}

	// Проверяем, поддерживает ли используемая версия go-openai дополнительные HTTP заголовки
	// Если нет, может потребоваться обновить библиотеку или использовать HTTP-клиент напрямую

	resp, err := o.client.CreateChatCompletion(ctx, req)

	if err != nil {
		log.Println("Ошибка при запросе в OpenRouter:", err)
		return nil, err
	}
	log.Println("Ответ получен от OpenRouter:", resp)

	content := resp.Choices[0].Message.Content
	log.Println("Ответ Molmo:", content)
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		log.Println("JSON не найден в ответе, пытаемся извлечь продукты из текста...")
		// Если JSON не найден, пытаемся создать его из текста
		lines := strings.Split(content, "\n")
		var items []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.Contains(line, "{") && !strings.Contains(line, "}") {
				items = append(items, line)
			}
		}

		if len(items) > 0 {
			return &RecognizedItems{Items: items}, nil
		}

		return nil, fmt.Errorf("нет продуктов на изображении")
	}

	jsonContent := content[jsonStart : jsonEnd+1]
	var recognized RecognizedItems
	if err := json.Unmarshal([]byte(jsonContent), &recognized); err != nil {
		return nil, err
	}

	if len(recognized.Items) == 0 {
		return nil, fmt.Errorf("нет продуктов на изображении")
	}

	return &recognized, nil
}
