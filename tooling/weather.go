package tooling

import (
	"context"
	"fmt"
	"focalors-go/service"
	"log/slog"

	"github.com/openai/openai-go"
)

// WeatherTool provides weather information
type WeatherTool struct {
	weather *service.WeatherService
}

type weatherArgs struct {
	Location string `json:"location"`
}

// NewWeatherTool creates a new weather tool
func NewWeatherTool(weather *service.WeatherService) *WeatherTool {
	return &WeatherTool{weather: weather}
}

func (w *WeatherTool) Name() string {
	return "get_weather"
}

func (w *WeatherTool) Definition() openai.FunctionDefinitionParam {
	return openai.FunctionDefinitionParam{
		Name:        "get_weather",
		Description: openai.String("Get current weather at the given location in China"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]string{
					"type":        "string",
					"description": "City or district name in Chinese, e.g. 北京, 深圳, 南山区",
				},
			},
			"required": []string{"location"},
		},
	}
}

func (w *WeatherTool) Execute(ctx context.Context, argsJSON string) (*ToolResult, error) {
	args, err := ParseArgs[weatherArgs](argsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to parse weather args: %w", err)
	}

	logger.Info("Getting weather data", slog.String("location", args.Location))

	weatherLives, err := w.weather.GetWeather(ctx, args.Location)
	if err != nil {
		logger.Error("Failed to get weather data", slog.String("location", args.Location), slog.Any("error", err))
		return NewToolResult("Failed to get weather data"), nil
	}
	if len(weatherLives) == 0 {
		return NewToolResult(fmt.Sprintf("No weather data found for %s", args.Location)), nil
	}

	live := weatherLives[0]
	text := fmt.Sprintf("%s %s: %s, 温度: %s°C, 风向: %s, 风力: %s级, 空气湿度: %s%%",
		live.Province,
		live.City,
		live.Weather,
		live.Temperature,
		live.WindDirection,
		live.WindPower,
		live.Humidity,
	)
	return NewToolResult(text), nil
}
