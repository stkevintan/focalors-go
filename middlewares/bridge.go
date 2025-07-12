package middlewares

import (
	"context"
	"fmt"
	"focalors-go/wechat"
	"focalors-go/yunzai"
	"log/slog"
	"regexp"
	"strings"
)

type bridgeMiddleware struct {
	*middlewareBase
	y           *yunzai.YunzaiClient
	avatarCache map[string]string
}

func NewBridgeMiddlewareFactory(y *yunzai.YunzaiClient) func(base *middlewareBase) Middleware {
	return func(base *middlewareBase) Middleware {
		return &bridgeMiddleware{
			middlewareBase: base,
			y:              y,
			avatarCache:    make(map[string]string),
		}
	}
}

func (b *bridgeMiddleware) Start() error {
	b.y.AddMessageHandler(b.onYunzaiMessage)
	return nil
}

func (b *bridgeMiddleware) OnMessage(ctx context.Context, msg *wechat.WechatMessage) bool {
	if !msg.IsText() || !regexp.MustCompile(`^[#*%]`).MatchString(msg.Text) {
		return false
	}
	b.updateAvatarCache(msg)

	userType := "group"
	if msg.ChatType == wechat.ChatTypePrivate {
		userType = "direct"
	}

	text := strings.TrimPrefix(msg.Text, "#!")

	sent := yunzai.Request{
		BotSelfId: "focalors",
		MsgId:     msg.MsgId,
		UserId:    msg.FromUserId,
		GroupId:   msg.FromGroupId,
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
	return false
}

func (b *bridgeMiddleware) logYunzaiMessage(msg *yunzai.Response) bool {
	logger.Info("Received Yunzai message",
		slog.String("BotId", msg.BotSelfId),
		slog.String("MsgId", msg.MsgId),
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
				b.SendText(msg, textContent)
			}
		case "image":
			imageContent, ok := content.Data.(string)
			if !ok {
				logger.Error("Failed to convert content to string", slog.Any("content", content))
				continue
			}
			b.SendImage(msg, imageContent)
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
	return false
}

func (b *bridgeMiddleware) createSender(message *wechat.WechatMessage) map[string]any {
	key := fmt.Sprintf("avatar:%s", message.FromUserId)
	if avatar, ok := b.avatarCache[key]; ok {
		return map[string]any{
			"avatar": avatar,
		}
	}
	avatar, err := b.redis.Get(key)
	if err != nil {
		logger.Error("Failed to get avatar result from Redis command", slog.String("key", key), slog.Any("error", err))
		return nil
	}

	if avatar != "" {
		b.avatarCache[key] = avatar // Update the cache
		return map[string]any{
			"avatar": avatar,
		}
	}
	return nil
}

func (b *bridgeMiddleware) updateAvatarCache(msg *wechat.WechatMessage) {
	var triggers = regexp.MustCompile(`^[#*%]更新(面板|头像)`)
	if msg.MsgType == wechat.TextMessage && triggers.MatchString(msg.Text) {
		res, err := b.GetUserContactDetails(msg.FromUserId)
		if err != nil {
			logger.Error("Failed to get contact details", slog.Any("error", err))
			return
		}
		for _, contact := range res.Data.ContactList {
			headUrl := contact.SmallHeadImgUrl
			key := "avatar:" + contact.UserName.Str
			b.avatarCache[key] = headUrl
			b.redis.Set(key, headUrl, 0)
		}
		b.SendText(msg, "头像已更新")
	}
}
