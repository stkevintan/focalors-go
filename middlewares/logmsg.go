package middlewares

import (
	"context"
	"fmt"
	"focalors-go/wechat"
	"log/slog"
)

type logMsgMiddleware struct {
	*middlewareBase
}

func NewLogMsgMiddleware(base *middlewareBase) Middleware {
	return &logMsgMiddleware{
		middlewareBase: base,
	}
}

func (l *logMsgMiddleware) OnMessage(ctx context.Context, msg *wechat.WechatMessage) bool {
	logger.Info("Received Wechat message",
		slog.String("FromUserId", msg.FromUserId),
		slog.String("FromGroupId", msg.FromGroupId),
		slog.String("ToUserId", msg.ToUserId),
		slog.String("MsgType", fmt.Sprintf("%d", msg.MsgType)),
		slog.String("Content", msg.Content),
	)
	return false
}
