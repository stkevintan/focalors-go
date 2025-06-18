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
		for _, content := range msg.Content {
			if content.Type == "text" {
				content := strings.Trim(content.Data.(string), " \n")
				if content == "" {
					continue
				}
				m.w.SendText(msg, content)
			}
			if content.Type == "image" {
				m.w.SendImage(msg, content.Data.(string))
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
