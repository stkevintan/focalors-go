package middlewares

import (
	"context"
	"fmt"
	"focalors-go/config"
	"focalors-go/db"
	"focalors-go/scheduler"
	"focalors-go/service"
	"focalors-go/slogger"
	"focalors-go/wechat"
	"log/slog"
)

var logger = slogger.New("middlewares")

type Middleware interface {
	OnMessage(ctx context.Context, msg *wechat.WechatMessage) bool
	Start() error
	Stop() error
}

type middlewareBase struct {
	*wechat.WechatClient
	access *service.AccessService
	redis  *db.Redis
	cron   *scheduler.CronTask
	cfg    *config.Config
}

func (m *middlewareBase) OnMessage(ctx context.Context, msg *wechat.WechatMessage) bool {
	return false
}

func (m *middlewareBase) Start() error {
	return nil
}

func (m *middlewareBase) Stop() error {
	return nil
}

func newBaseMiddleware(
	w *wechat.WechatClient,
	redis *db.Redis,
	cron *scheduler.CronTask,
	cfg *config.Config,
) *middlewareBase {
	return &middlewareBase{
		access:       service.NewAccessService(w, redis, cfg.App.Admin),
		WechatClient: w,
		redis:        redis,
		cron:         cron,
		cfg:          cfg,
	}
}

type RootMiddleware struct {
	base *middlewareBase
	// sync lock?
	middlewares []Middleware
}

func NewRootMiddleware(
	w *wechat.WechatClient,
	redis *db.Redis,
	cron *scheduler.CronTask,
	cfg *config.Config,
) *RootMiddleware {
	return &RootMiddleware{
		base: newBaseMiddleware(w, redis, cron, cfg),
	}
}

func (r *RootMiddleware) AddMiddlewares(middlewares ...func(m *middlewareBase) Middleware) {
	for _, mw := range middlewares {
		instance := mw(r.base)
		if instance != nil {
			r.middlewares = append(r.middlewares, instance)
		}
	}
}

func (r *RootMiddleware) Start() error {
	for _, mw := range r.middlewares {
		// register middleware
		r.base.AddMessageHandler(mw.OnMessage)
		if err := mw.Start(); err != nil {
			return err
		}
		logger.Info("Middleware started", slog.String("type", fmt.Sprintf("%T", mw)))
	}
	return nil
}

func (r *RootMiddleware) Stop() error {
	for _, mw := range r.middlewares {
		if err := mw.Stop(); err != nil {
			return err
		}
	}
	return nil
}
