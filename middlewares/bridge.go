package middlewares

import (
	"fmt"
	"focalors-go/wechat"
	"focalors-go/yunzai"
	"log/slog"
	"regexp"
	"strings"
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
				}
				logger.Debug("Sending message to yunzai", slog.Any("request", sent))
				m.y.Send(sent)
			}
			return false
		})
}
