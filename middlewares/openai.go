package middlewares

import (
	"context"
	"encoding/json"
	"fmt"
	"focalors-go/client"
	"focalors-go/service"
	"log/slog"
	"slices"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
)

type OpenAIMiddleware struct {
	*MiddlewareContext
	openai  *openai.Client
	weather *service.WeatherService
}

func NewOpenAIMiddleware(base *MiddlewareContext) Middleware {
	if base.cfg.OpenAI.APIKey == "" || base.cfg.OpenAI.Endpoint == "" {
		return nil
	}

	client := openai.NewClient(
		azure.WithEndpoint(base.cfg.OpenAI.Endpoint, base.cfg.OpenAI.APIVersion),
		azure.WithAPIKey(base.cfg.OpenAI.APIKey),
	)

	return &OpenAIMiddleware{
		MiddlewareContext: base,
		openai:            &client,
		weather:           service.NewWeatherService(&base.cfg.Weather),
	}
}

func (o *OpenAIMiddleware) OnMessage(ctx context.Context, msg client.GenericMessage) bool {
	if !msg.IsText() || msg.GetText() == "" || !msg.IsMentioned() {
		return false
	}
	if ok, _ := o.access.HasAccess(msg.GetTarget(), service.GPTAccess); !ok {
		return false
	}
	content := msg.GetText()

	// get thread
	messages := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage(content),
	}
	if referMessage, ok := msg.GetReferMessage(); ok {
		if referMessage != nil && referMessage.GetText() != "" {
			if referMessage.GetUserId() == o.client.GetSelfUserId() {
				messages = append(messages, openai.AssistantMessage(referMessage.GetText()))
			} else {
				messages = append(messages, openai.UserMessage(referMessage.GetText()))
			}
		}
	}
	slices.Reverse(messages)
	response, err := o.onTextMode(ctx, messages)
	if err != nil {
		o.client.SendText(msg, fmt.Sprintf("糟糕，%s", err.Error()))
	}
	o.client.SendText(msg, response)
	return true
}

func (o *OpenAIMiddleware) onTextMode(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) (string, error) {
	logger.Info("Sending message to OpenAI", slog.Any("messages", messages))
	params := openai.ChatCompletionNewParams{
		Model:     openai.ChatModel(o.cfg.OpenAI.Deployment),
		Messages:  messages,
		MaxTokens: openai.Int(2048),
		Tools: []openai.ChatCompletionToolParam{
			{
				Function: openai.FunctionDefinitionParam{
					Name:        "get_weather",
					Description: openai.String("Get weather at the given location"),
					Parameters: openai.FunctionParameters{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]string{
								"type": "string",
							},
						},
						"required": []string{"location"},
					},
				},
			},
		},
	}

	completion, err := o.openai.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}
	toolCalls := completion.Choices[0].Message.ToolCalls

	// Return early if there are no tool calls
	if len(toolCalls) == 0 {
		return completion.Choices[0].Message.Content, nil
	}

	// If there is a was a function call, continue the conversation
	params.Messages = append(params.Messages, completion.Choices[0].Message.ToParam())
	for _, toolCall := range toolCalls {
		if toolCall.Function.Name == "get_weather" {
			// Extract the location from the function call arguments
			var args map[string]interface{}
			err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			if err != nil {
				return "", err
			}
			location := args["location"].(string)

			// Simulate getting weather data
			weatherData := o.getWeather(ctx, location)

			params.Messages = append(params.Messages, openai.ToolMessage(weatherData, toolCall.ID))
		}
	}

	completion, err = o.openai.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", err
	}
	return completion.Choices[0].Message.Content, nil
}

func (o *OpenAIMiddleware) getWeather(ctx context.Context, location string) string {
	logger.Info("Getting weather data", slog.String("location", location))
	// Simulate getting weather data
	weatherLives, err := o.weather.GetWeather(ctx, location)
	if err != nil {
		logger.Error("Failed to get weather data", slog.String("location", location), slog.Any("error", err))
		return "Failed to get weather data"
	}
	if len(weatherLives) == 0 {
		return fmt.Sprintf("No weather data found for %s", location)
	}
	return fmt.Sprintf("%s, 温度: %s, 风向: %s, 风力: %s, 空气湿度: %s",
		weatherLives[0].Weather,
		weatherLives[0].Temperature,
		weatherLives[0].WindDirection,
		weatherLives[0].WindPower,
		weatherLives[0].Humidity)
}
