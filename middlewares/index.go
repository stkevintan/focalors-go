package middlewares

import (
	"context"
	"fmt"
	"focalors-go/client"
	"focalors-go/config"
	"focalors-go/db"
	"focalors-go/scheduler"
	"focalors-go/service"
	"focalors-go/slogger"
	"log/slog"
)

var logger = slogger.New("middlewares")

type Middleware interface {
	OnMessage(ctx context.Context, msg client.GenericMessage) bool
	Start() error
	Stop() error
}

type MiddlewareContext struct {
	redis  *db.Redis
	cron   *scheduler.CronTask
	cfg    *config.Config
	access *service.AccessService
	ctx    context.Context
	client client.GenericClient
}

func NewMiddlewareContext(ctx context.Context, client client.GenericClient, cfg *config.Config, redis *db.Redis) *MiddlewareContext {
	cron := scheduler.NewCronTask(redis)
	access := service.NewAccessService(redis, cfg.App.Admin)
	// init
	cron.Start()
	return &MiddlewareContext{
		redis:  redis,
		cron:   cron,
		cfg:    cfg,
		access: access,
		ctx:    ctx,
		client: client,
	}
}

func (mctx *MiddlewareContext) Close() {
	mctx.cron.Stop()
}

func (m *MiddlewareContext) OnMessage(ctx context.Context, msg client.GenericMessage) bool {
	return false
}

func (m *MiddlewareContext) Start() error {
	return nil
}

func (m *MiddlewareContext) Stop() error {
	return nil
}

type RootMiddleware struct {
	*MiddlewareContext
	// sync lock?
	middlewares []Middleware
}

func NewRootMiddleware(
	mctx *MiddlewareContext,
) *RootMiddleware {
	return &RootMiddleware{
		MiddlewareContext: mctx,
	}
}

func (r *RootMiddleware) AddMiddlewares(middlewares ...func(m *MiddlewareContext) Middleware) {
	for _, mw := range middlewares {
		instance := mw(r.MiddlewareContext)
		if instance != nil {
			r.middlewares = append(r.middlewares, instance)
		}
	}
}

func (r *RootMiddleware) Start() error {
	for _, mw := range r.middlewares {
		if r.client != nil {
			r.client.AddMessageHandler(mw.OnMessage)
		}
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
