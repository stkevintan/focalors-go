package middlewares

import (
	"context"
	"encoding/json"
	"fmt"
	"focalors-go/service"
	"focalors-go/wechat"
	"log/slog"
	"slices"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
)

type OpenAIMiddleware struct {
	*middlewareBase
	client  *openai.Client
	weather *service.WeatherService
}

func NewOpenAIMiddleware(base *middlewareBase) Middleware {
	if base.cfg.OpenAI.APIKey == "" || base.cfg.OpenAI.Endpoint == "" {
		return nil
	}

	client := openai.NewClient(
		azure.WithEndpoint(base.cfg.OpenAI.Endpoint, base.cfg.OpenAI.APIVersion),
		azure.WithAPIKey(base.cfg.OpenAI.APIKey),
	)

	return &OpenAIMiddleware{
		middlewareBase: base,
		client:         &client,
		weather:        service.NewWeatherService(&base.cfg.Weather),
	}
}

func (o *OpenAIMiddleware) OnMessage(ctx context.Context, msg *wechat.WechatMessage) bool {
	if fs := msg.ToFlagSet("gpt"); fs != nil {
		if ok, _ := o.access.HasAccess(msg, service.GPTAccess); !ok {
			return false
		}

		// imageMode := fs.Bool("img", false, "Whether to use image mode")
		if help := fs.Parse(); help != "" {
			o.SendText(msg, help)
			return true
		}
		content := fs.Rest()
		// get thread
		messages := []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(content),
		}
		if msg.MsgType == wechat.ReferMessage {
			referMessage := o.GetReferMessage(msg)
			if referMessage != nil && referMessage.Text != "" {
				if referMessage.IsSelfMsg {
					messages = append(messages, openai.AssistantMessage(referMessage.Text))
				} else {
					messages = append(messages, openai.UserMessage(referMessage.Text))
				}
			}
		}
		slices.Reverse(messages)
		response, err := o.onTextMode(ctx, messages)
		if err != nil {
			o.SendText(msg, fmt.Sprintf("糟糕，%s", err.Error()))
		}
		o.SendText(msg, response)
		return true
	}
	return false
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

	completion, err := o.client.Chat.Completions.New(ctx, params)
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

	completion, err = o.client.Chat.Completions.New(ctx, params)
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
