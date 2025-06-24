package middlewares

import (
	"fmt"
	"focalors-go/wechat"
	"focalors-go/yunzai"
	"log/slog"
)

type logMsgMiddleware struct {
	*MiddlewareBase
	y *yunzai.YunzaiClient
}

func newLogMsgMiddleware(base *MiddlewareBase, y *yunzai.YunzaiClient) *logMsgMiddleware {
	return &logMsgMiddleware{
		MiddlewareBase: base,
		y:              y,
	}
}

func (l *logMsgMiddleware) OnRegister() error {
	l.y.AddMessageHandler(l.OnYunzaiMessage)
	return nil
}

func (l *logMsgMiddleware) OnMessage(msg *wechat.WechatMessage) bool {
	logger.Info("Received Wechat message",
		slog.String("FromUserId", msg.FromUserId),
		slog.String("FromGroupId", msg.FromGroupId),
		slog.String("ToUserId", msg.ToUserId),
		slog.String("MsgType", fmt.Sprintf("%d", msg.MsgType)),
		slog.String("Content", msg.PushContent),
	)
	return false
}

func (l *logMsgMiddleware) OnYunzaiMessage(msg *yunzai.Response) bool {
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
