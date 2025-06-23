package middlewares

import (
	"fmt"
	"focalors-go/config"
	"focalors-go/db"
	"focalors-go/slogger"
	"focalors-go/wechat"
	"focalors-go/yunzai"
	"log/slog"
)

var logger = slogger.New("middlewares")

type MiddlewareBase struct {
	cfg   *config.Config
	w     *wechat.WechatClient
	redis *db.Redis
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
	m.w.AddMessageHandler(m.OnWechatMessage)
	return nil
}
func (m *MiddlewareBase) OnStop() error {
	return nil
}

type Middlewares struct {
	cron       *CronUtil
	middleware []Middleware
}

func New(w *wechat.WechatClient, y *yunzai.YunzaiClient, redis *db.Redis, cfg *config.Config) *Middlewares {
	cron := NewCronUtil(redis)
	m := &MiddlewareBase{
		cfg:   cfg,
		w:     w,
		redis: redis,
	}
	return &Middlewares{
		cron: cron,
		middleware: []Middleware{
			NewLogMsgMiddleware(m, y),
			NewAdminMiddleware(m, cron),
			NewJiadanMiddleware(m, cron),
			NewBridgeMiddleware(m, y),
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

	for _, mw := range m.middleware {
		if err := mw.OnStop(); err != nil {
			logger.Error("Failed to stop middleware", slog.Any("error", err))
		} else {
			logger.Info("Middleware stopped successfully", slog.String("type", fmt.Sprintf("%T", mw)))
		}
	}
}
