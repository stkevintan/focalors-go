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
	OnWechatMessage(msg *wechat.WechatMessage) bool
	OnStart() error
	OnStop() error
}

func (m *MiddlewareBase) OnWechatMessage(msg *wechat.WechatMessage) bool {
	return false
}

func (m *MiddlewareBase) OnYunzaiMessage(msg *yunzai.Response) bool {
	return false
}

func (m *MiddlewareBase) OnStart() error {
	m.AddMessageHandler(m.OnWechatMessage)
	return nil
}
func (m *MiddlewareBase) OnStop() error {
	return nil
}

type Middlewares struct {
	cron       *scheduler.CronTask
	middleware []Middleware
	redis      *db.Redis
}

func New(ctx context.Context, w *wechat.WechatClient, y *yunzai.YunzaiClient, cfg *config.Config) *Middlewares {
	redis := db.NewRedis(ctx, &cfg.Redis)
	cron := scheduler.NewCronTask(redis)
	m := &MiddlewareBase{
		cfg:          cfg,
		WechatClient: w,
	}
	return &Middlewares{
		cron:  cron,
		redis: redis,
		middleware: []Middleware{
			newLogMsgMiddleware(m, y),
			newAdminMiddleware(m, cron),
			newJiadanMiddleware(m, cron, redis),
			newBridgeMiddleware(m, y, redis),
		},
	}
}

func (m *Middlewares) Start() {
	for _, mw := range m.middleware {
		if err := mw.OnStart(); err != nil {
			logger.Error("Failed to start middleware", slog.Any("error", err))
		} else {
			logger.Info("Middleware started successfully", slog.String("type", fmt.Sprintf("%T", mw)))
		}
	}
	m.cron.Start()
}

func (m *Middlewares) Stop() {
	m.cron.Stop()
	m.redis.Close()
	for _, mw := range m.middleware {
		if err := mw.OnStop(); err != nil {
			logger.Error("Failed to stop middleware", slog.Any("error", err))
		} else {
			logger.Info("Middleware stopped successfully", slog.String("type", fmt.Sprintf("%T", mw)))
		}
	}
}
