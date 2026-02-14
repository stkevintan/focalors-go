package middlewares

import (
	"context"
	"fmt"
	"focalors-go/contract"
	"focalors-go/db"
	"focalors-go/service"
	"focalors-go/tooling"
	"log/slog"
	"slices"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/azure"
)

type OpenAIMiddleware struct {
	*MiddlewareContext
	openai   *openai.Client
	registry *tooling.Registry
}

func NewOpenAIMiddleware(base *MiddlewareContext) Middleware {
	if base.cfg.OpenAI.APIKey == "" || base.cfg.OpenAI.Endpoint == "" {
		return nil
	}

	client := openai.NewClient(
		azure.WithEndpoint(base.cfg.OpenAI.Endpoint, base.cfg.OpenAI.APIVersion),
		azure.WithAPIKey(base.cfg.OpenAI.APIKey),
	)

	// Create tool registry and register tools
	registry := tooling.NewRegistry()
	registry.Register(tooling.NewWeatherTool(service.NewWeatherService(&base.cfg.Weather)))
	jiandanStore := db.NewJiandanStore(base.redis)
	registry.Register(tooling.NewJiadanTool(service.NewJiadanService(jiandanStore)))

	return &OpenAIMiddleware{
		MiddlewareContext: base,
		openai:            &client,
		registry:          registry,
	}
}

func (o *OpenAIMiddleware) OnMessage(ctx context.Context, msg contract.GenericMessage) bool {
	logger.Info("OAI check", slog.Bool("isText", msg.IsText()), slog.String("text", msg.GetText()), slog.Bool("isMentioned", msg.IsMentioned()))

	if !msg.IsText() || msg.GetText() == "" || !msg.IsMentioned() {
		return false
	}

	if ok, _ := o.access.HasAccess(msg.GetTarget(), service.GPTAccess); !ok {
		logger.Info("User does not have access to GPT", slog.String("target", msg.GetTarget()))
		return false
	}

	content := msg.GetText()
	logger.Info("Received message for OpenAI", slog.String("content", content))

	sender := o.SendPendingReply(msg)

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
	// Add target to context for tools
	toolCtx := tooling.WithTarget(ctx, msg.GetTarget())
	response, contents, err := o.onTextMode(toolCtx, messages)

	if err != nil {
		sender.SendMarkdown(fmt.Sprintf("糟糕，%s", err.Error()))
		return true
	}

	// Build card with response and any content from tools
	card := contract.NewCardBuilder()
	// special handling for jiandan tool to avoid duplicate text since the main response may already contain post info, and the tool content is mainly for images
	if !slices.ContainsFunc(contents, func(c tooling.Content) bool { return c.ToolName == "jiandan_top" }) {
		card.AddMarkdown(response)
	}
	for _, content := range contents {
		switch content.Type {
		case tooling.ContentText:
			card.AddMarkdown(content.Text)
		case tooling.ContentImage:
			if content.Image == "" {
				continue
			}
			// Debug: log first 100 chars of image data to see if it's base64 or URL
			preview := content.Image
			if len(preview) > 100 {
				preview = preview[:100]
			}
			logger.Debug("uploading tool image", slog.String("preview", preview), slog.Int("len", len(content.Image)))

			imageKey, err := o.client.UploadImage(content.Image)
			if err != nil {
				logger.Warn("failed to upload tool image", slog.Any("error", err))
				continue
			}
			if imageKey == "" {
				continue
			}
			logger.Debug("uploaded tool image", slog.String("imageKey", imageKey))
			card.AddImage(imageKey, content.AltText)
		default:
			logger.Warn("unimplemented content type", slog.Any("type", content.Type), slog.String("tool", content.ToolName))
		}
	}
	logger.Debug("sending response card", slog.Any("card", card))
	sender.SendRichCard(card)
	return true
}

func (o *OpenAIMiddleware) onTextMode(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) (string, []tooling.Content, error) {
	logger.Info("Sending message to OpenAI", slog.Any("messages", messages))
	params := openai.ChatCompletionNewParams{
		Model:     openai.ChatModel(o.cfg.OpenAI.Deployment),
		Messages:  messages,
		MaxTokens: openai.Int(2048),
		Tools:     o.registry.Definitions(),
	}

	completion, err := o.openai.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", nil, err
	}
	toolCalls := completion.Choices[0].Message.ToolCalls

	// Return early if there are no tool calls
	if len(toolCalls) == 0 {
		return completion.Choices[0].Message.Content, nil, nil
	}

	// Collect contents from tool results
	var allContents []tooling.Content

	// If there were tool calls, execute them and continue the conversation
	params.Messages = append(params.Messages, completion.Choices[0].Message.ToParam())
	for _, toolCall := range toolCalls {
		result, err := o.registry.Execute(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
		if err != nil {
			logger.Error("Tool execution failed", slog.String("tool", toolCall.Function.Name), slog.Any("error", err))
			result = &tooling.ToolResult{Text: fmt.Sprintf("Tool error: %s", err.Error())}
		}
		params.Messages = append(params.Messages, openai.ToolMessage(result.Text, toolCall.ID))
		allContents = append(allContents, result.Contents...)
	}

	completion, err = o.openai.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", nil, err
	}
	return completion.Choices[0].Message.Content, allContents, nil
}
