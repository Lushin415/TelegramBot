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

type OpenAIVision struct {
	client *openai.Client
	logger *zap.Logger
}

type RecognizedItems struct {
	Items []string `json:"items"`
}

func NewOpenAIVision(apiKey string, logger *zap.Logger) *OpenAIVision {
	return &OpenAIVision{
		client: openai.NewClient(apiKey),
		logger: logger,
	}
}

func (o *OpenAIVision) RecognizeProductsFromImage(ctx context.Context, imageData io.Reader) (*RecognizedItems, error) {
	data, err := io.ReadAll(imageData)
	if err != nil {
		return nil, err
	}

	base64Image := base64.StdEncoding.EncodeToString(data)

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
							Text: "Определи все продукты на этом изображении. Верни только JSON с массивом {'items': [продукты]}.",
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
		},
	)

	if err != nil {
		return nil, err
	}

	content := resp.Choices[0].Message.Content
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
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
