package middlewares

import (
	"errors"
	"fmt"
	"focalors-go/wechat"
	"focalors-go/yunzai"
	"log/slog"
	"regexp"
	"strings"

	"github.com/redis/go-redis/v9"
)

func (m *Middlewares) AddBridge() {
	var prefixRegex = regexp.MustCompile(`^[#*%]`)

	createSender := func(message *wechat.WechatMessage) map[string]any {
		key := fmt.Sprintf("avatar:%s", message.FromUserId)
		if avatar, ok := m.avatarCache[key]; ok {
			return map[string]any{
				"avatar": avatar,
			}
		}
		cmd := m.redis.Get(m.ctx, key)
		err := cmd.Err()

		if err != nil {
			if errors.Is(err, redis.Nil) {
				logger.Debug("Avatar not found in Redis", slog.String("key", key))
			} else {
				logger.Error("Failed to get avatar from Redis", slog.String("key", key), slog.Any("error", err))
			}
			return nil
		}

		avatar, err := cmd.Result()
		if err != nil {
			logger.Error("Failed to get avatar result from Redis command", slog.String("key", key), slog.Any("error", err))
			return nil
		}

		if avatar != "" {
			m.avatarCache[key] = avatar // Update the cache
			return map[string]any{
				"avatar": avatar,
			}
		}
		return nil
	}

	// yunzai message => wechat
	m.y.AddMessageHandler(func(msg *yunzai.Response) bool {
		queue := make([]yunzai.MessageContent, 0, len(msg.Content))
		queue = append(queue, msg.Content...)
		for len(queue) > 0 {
			content := queue[0]
			queue = queue[1:]
			switch content.Type {
			case "text":
				// {"type":"text","data":false}
				textContent, ok := content.Data.(string)
				if !ok {
					logger.Error("Failed to convert content to string", slog.Any("content", content))
					continue
				}
				textContent = strings.Trim(textContent, " \n")
				if textContent != "" {
					m.w.SendText(msg, textContent)
				}
			case "image":
				imageContent, ok := content.Data.(string)
				if !ok {
					logger.Error("Failed to convert content to string", slog.Any("content", content))
					continue
				}
				m.w.SendImage(msg, imageContent)
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
						}
					}
				}
			default:
				logger.Warn("Unsupported message type", slog.Any("content", content))
			}
		}
		return false
	})

	// wechat message => yunzai
	m.w.AddMessageHandler(
		func(message *wechat.WechatMessage) bool {
			if message.MsgType == wechat.TextMessage && prefixRegex.MatchString(message.Content) {
				userType := "group"
				if message.ChatType == wechat.ChatTypePrivate {
					userType = "direct"
				}

				message.Content = strings.TrimPrefix(message.Content, "#!")

				sent := yunzai.Request{
					BotSelfId: "focalors",
					MsgId:     fmt.Sprintf("%d", message.MsgId),
					UserId:    message.FromUserId,
					GroupId:   message.FromGroupId,
					UserPM:    0,
					UserType:  userType,
					Content: []yunzai.MessageContent{
						{
							Type: "text",
							Data: message.Content,
						},
					},
					Sender: createSender(message),
				}
				logger.Debug("Sending message to yunzai", slog.Any("request", sent))
				m.y.Send(sent)
			}
			return false
		})

}
