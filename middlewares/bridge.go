package middlewares

import (
	"context"
	"focalors-go/client"
	"focalors-go/db"
	"focalors-go/yunzai"
	"log/slog"
	"regexp"
	"strings"
)

type bridgeMiddleware struct {
	*MiddlewareContext
	y           *yunzai.YunzaiClient
	avatarStore *db.AvatarStore
}

func NewBridgeMiddleware(base *MiddlewareContext) Middleware {
	// create new yunzai client
	y := yunzai.NewYunzai(base.cfg)
	return &bridgeMiddleware{
		MiddlewareContext: base,
		y:                 y,
		avatarStore:       db.NewAvatarStore(base.redis),
	}
}

func (b *bridgeMiddleware) Start() error {
	go b.y.Start(b.ctx)
	b.y.AddMessageHandler(b.onYunzaiMessage)
	return nil
}

func (b *bridgeMiddleware) OnMessage(ctx context.Context, msg client.GenericMessage) bool {
	if !msg.IsText() || !regexp.MustCompile(`^[#*%]`).MatchString(msg.GetText()) {
		return false
	}

	b.updateAvatarCache(msg)

	userType := "direct"
	if msg.IsGroup() {
		userType = "group"
	}

	text := strings.TrimPrefix(msg.GetText(), "#!")

	sent := yunzai.Request{
		BotSelfId: "focalors",
		MsgId:     msg.GetId(),
		UserId:    msg.GetUserId(),
		GroupId:   msg.GetGroupId(),
		UserPM:    0,
		UserType:  userType,
		Content: []yunzai.MessageContent{
			{
				Type: "text",
				Data: text,
			},
		},
		Sender: b.createSender(msg),
	}
	logger.Debug("Sending message to yunzai", slog.Any("request", sent))
	b.y.Send(sent)
	return true
}

func (b *bridgeMiddleware) logYunzaiMessage(msg *yunzai.Response) bool {
	logger.Info("Received Yunzai message",
		slog.String("BotId", msg.BotSelfId),
		slog.String("TargetId", msg.TargetId),
	)
	for _, content := range msg.Content {
		logger.Info("ContentType", slog.String("Type", content.Type))
		if content.Type == "image" && content.Data != nil {
			if dataStr, ok := content.Data.(string); ok && len(dataStr) > 10 {
				logger.Info("ContentData (image preview)", slog.String("Data", dataStr[:10]))
			} else {
				logger.Info("ContentData (image)", slog.Any("Data", content.Data))
			}
			continue
		}
		logger.Info("ContentData", slog.Any("Data", content.Data))
	}
	return false
}

func (b *bridgeMiddleware) onYunzaiMessage(ctx context.Context, msg *yunzai.Response) bool {
	b.logYunzaiMessage(msg)
	// its rare to has extra message push from yunzai
	queue := make([]yunzai.MessageContent, len(msg.Content))
	copy(queue, msg.Content)
	front := 0
	card := client.NewCardBuilder()
	for front < len(queue) {
		content := queue[front]
		front++
		switch content.Type {
		case "text":
			textContent, ok := content.Data.(string)
			if !ok {
				logger.Error("Failed to convert content to string", slog.Any("content", content))
				continue
			}
			textContent = strings.Trim(textContent, " \n")
			if textContent != "" {
				card.AddMarkdown(textContent)
			}
		case "image":
			imageContent, ok := content.Data.(string)
			if !ok {
				logger.Error("Failed to convert content to string", slog.Any("content", content))
				continue
			}
			// b.SendImage(msg, imageContent)
			if key, err := b.client.UploadImage(imageContent); err != nil {
				logger.Error("Failed to upload image", slog.Any("error", err))
				card.AddMarkdown("*上传图片失败*")
			} else {
				card.AddImage(key, "")
			}
		case "node":
			nodeContent, ok := content.Data.([]any)
			if !ok {
				logger.Error("Failed to convert content to []any", slog.Any("content", content))
				continue
			}
			for _, node := range nodeContent {
				if nodeMap, ok := node.(map[string]any); ok {
					if msgType, ok := nodeMap["type"].(string); ok {
						queue = append(queue, yunzai.MessageContent{
							Type: msgType,
							Data: nodeMap["data"],
						})
						logger.Debug("Sending message to wechat", slog.Any("node", node), slog.Any("queue", queue))
					} else {
						logger.Error("Failed to get message type", slog.Any("node", node))
					}
				}
			}
		default:
			logger.Warn("Unsupported message type", slog.Any("content", content))
		}
	}
	if len(card.Elements) > 0 {
		b.client.SendRichCard(msg, card)
	}
	return false
}

func (b *bridgeMiddleware) createSender(message client.GenericMessage) map[string]any {
	if avatar, ok := b.avatarStore.Get(message.GetUserId()); ok {
		return map[string]any{
			"avatar": avatar,
		}
	}
	return nil
}

func (b *bridgeMiddleware) updateAvatarCache(msg client.GenericMessage) {
	var triggers = regexp.MustCompile(`^[#*%]更新(面板|头像)`)
	if msg.IsText() && triggers.MatchString(msg.GetText()) {
		contacts, err := b.client.GetContactDetail(msg.GetUserId())
		if err != nil {
			logger.Error("Failed to get contact details", slog.Any("error", err))
			return
		}
		for _, contact := range contacts {
			b.avatarStore.Save(contact.Username(), contact.AvatarUrl())
		}
		b.SendText(msg, "头像已更新")
	}
}
