package middlewares

import (
	"context"
	"fmt"
	"focalors-go/config"
	"focalors-go/db"
	"focalors-go/scheduler"
	"focalors-go/slogger"
	"focalors-go/wechat"
	"focalors-go/yunzai"
	"log/slog"
)

var logger = slogger.New("middlewares")

type MiddlewareBase struct {
	*wechat.WechatClient
	cfg *config.Config
}

type Middleware interface {
	OnMessage(ctx context.Context, msg *wechat.WechatMessage) bool
	OnRegister() error
	OnStart() error
	OnStop() error
}

func (m *MiddlewareBase) OnMessage(ctx context.Context, msg *wechat.WechatMessage) bool {
	return false
}

func (m *MiddlewareBase) OnStart() error {
	return nil
}

func (m *MiddlewareBase) OnStop() error {
	return nil
}

func (m *MiddlewareBase) OnRegister() error {
	return nil
}

type Middlewares struct {
	middlewares []Middleware
}

func New(
	redis *db.Redis,
	cron *scheduler.CronTask,
	w *wechat.WechatClient,
	y *yunzai.YunzaiClient,
	cfg *config.Config,
) *Middlewares {
	m := &MiddlewareBase{
		WechatClient: w,
		cfg:          cfg,
	}
	middlewares := []Middleware{
		newLogMsgMiddleware(m, y),
		newAdminMiddleware(m, cron),
		newJiadanMiddleware(m, cron, redis),
		newOpenAIMiddleware(m, redis),
		newBridgeMiddleware(m, y, redis),
	}
	// remove nil middlewares
	var i int
	for _, mw := range middlewares {
		if mw != nil {
			middlewares[i] = mw
			i++
		}
	}
	middlewares = middlewares[:i]

	// register
	for _, mw := range middlewares {
		w.AddMessageHandler(mw.OnMessage)
		if err := mw.OnRegister(); err != nil {
			logger.Error("Failed to register middleware", slog.Any("error", err))
			continue
		}
		logger.Info("Middleware registered", slog.String("type", fmt.Sprintf("%T", mw)))
	}
	return &Middlewares{
		middlewares: middlewares,
	}
}

func (m *Middlewares) Start() {
	for _, mw := range m.middlewares {
		if err := mw.OnStart(); err != nil {
			logger.Error("Failed to start middleware", slog.Any("error", err))
			continue
		}
		logger.Info("Middleware started successfully", slog.String("type", fmt.Sprintf("%T", mw)))
	}
}

func (m *Middlewares) Stop() {
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		if err := m.middlewares[i].OnStop(); err != nil {
			logger.Error("Failed to stop middleware", slog.Any("error", err))
			continue
		}
		logger.Info("Middleware stopped successfully", slog.String("type", fmt.Sprintf("%T", m.middlewares[i])))
	}
}
