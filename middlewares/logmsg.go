package middlewares

import (
	"context"
	"focalors-go/client"
	"log/slog"
)

type logMsgMiddleware struct{}

func NewLogMsgMiddleware(_context *MiddlewareContext) Middleware {
	return &logMsgMiddleware{}
}

func (l *logMsgMiddleware) OnMessage(ctx context.Context, msg client.GenericMessage) bool {
	logger.Info("Received Wechat message",
		slog.String("FromUserId", msg.GetUserId()),
		slog.String("FromGroupId", msg.GetGroupId()),
		// slog.String("ToUserId", msg.GetToUserId()),
		// slog.String("MsgType", fmt.Sprintf("%d", msg.GetMsgType())),
		slog.String("Content", msg.GetContent()),
	)
	return false
}

func (l *logMsgMiddleware) Start() error {
	return nil
}

func (l *logMsgMiddleware) Stop() error {
	return nil
}
