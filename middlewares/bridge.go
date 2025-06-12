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

var prefixRegex = regexp.MustCompile(`^[#*%]`)

func (m *Middlewares) AddBridge() {
	// yunzai message => wechat
	m.y.AddMessageHandler(func(msg yunzai.Response) bool {
		for _, content := range msg.Content {
			if content.Type == "text" {
				content := strings.Trim(content.Data.(string), " \n")
				if content == "" {
					continue
				}
				m.w.SendTextMessage([]wechat.MessageItem{
					{
						ToUserName:  msg.TargetId,
						TextContent: content,
						MsgType:     1,
						// TODO
						// AtWxIDList: []string{},
					},
				})
			}
			if content.Type == "image" {
				m.w.SendImageNewMessage([]wechat.MessageItem{
					{
						ToUserName:   msg.TargetId,
						ImageContent: strings.TrimPrefix(content.Data.(string), "base64://"),
						MsgType:      2,
					},
				})
			}
		}
		return false
	})

	// wechat message => yunzai
	m.w.AddMessageHandler(
		func(message wechat.WechatMessage) bool {
			if message.MsgType == wechat.TextMessage && prefixRegex.MatchString(message.Content) {
				userType := "group"
				if message.ChatType == wechat.ChatTypePrivate {
					userType = "direct"
				}

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
					Sender: m.createSender(message),
				}
				logger.Debug("Sending message to yunzai", slog.Any("request", sent))
				m.y.Send(sent)
			}
			return false
		})
}

func (m *Middlewares) createSender(message wechat.WechatMessage) map[string]any {
	key := fmt.Sprintf("avatar:%s", message.FromUserId)
	cmd := m.redis.Get(m.ctx, key)
	err := cmd.Err()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Key does not exist, this is not an application error.
			// The original logic handles this by returning nil if avatar is empty.
			logger.Debug("Avatar not found in Redis", slog.String("key", key))
		} else {
			// Some other Redis error occurred
			logger.Error("Failed to get avatar from Redis", slog.String("key", key), slog.Any("error", err))
		}
		return nil // Return nil if key not found or on error
	}

	avatar, err := cmd.Result() // Or cmd.Val()
	if err != nil {
		// This case should ideally be covered by cmd.Err() above,
		// but it's good practice to check the error from Result/Val as well.
		logger.Error("Failed to get avatar result from Redis command", slog.String("key", key), slog.Any("error", err))
		return nil
	}

	if avatar != "" {
		return map[string]any{
			"avatar": avatar,
		}
	}
	return nil
}
