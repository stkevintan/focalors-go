package middlewares

import (
	"fmt"
	"focalors-go/wechat"
	"focalors-go/yunzai"
	"log/slog"
)

func (m *Middlewares) AddLogMsg() {
	m.y.AddMessageHandler(func(message yunzai.Response) bool {
		logger.Info("Received Yunzai message",
			slog.String("BotId", message.BotSelfId),
			slog.String("MsgId", message.MsgId),
			slog.String("TargetId", message.TargetId),
		)
		for _, content := range message.Content {
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
	})

	m.w.AddMessageHandler(func(message wechat.WechatMessage) bool {
		logger.Info("Received message",
			slog.String("FromUserId", message.FromUserId),
			slog.String("FromGroupId", message.FromGroupId),
			slog.String("ToUserId", message.ToUserId),
			slog.String("MsgType", fmt.Sprintf("%d", message.MsgType)),
			slog.String("Content", message.PushContent),
		)
		return false
	})
}
