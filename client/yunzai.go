package client

import (
	"context"
	cfg "focalors-go/config"
	"log/slog"
	"sync"
	"time"
)

type Yunzai struct {
	ws       *WebSocketClient
	handlers []func(msg Response) bool
}

func NewYunzai(cfg *cfg.YunzaiConfig) *Yunzai {
	return &Yunzai{
		ws: New(cfg.Server),
	}
}

func (y *Yunzai) AddMessageHandler(handler func(msg Response) bool) {
	y.handlers = append(y.handlers, handler)
}

func (y *Yunzai) Start(ctx context.Context) {
	ready := make(chan struct{})
	var readyOnce sync.Once
	go func() {
		for {
			select {
			case <-ctx.Done():
				logger.Info("Context cancelled, exiting")
				y.ws.Close()
				return
			case <-time.After(time.Duration(3) * time.Millisecond * 1000):
				// Try to connect if not connected
				err := y.ws.Connect()
				if err != nil {
					logger.Error("Failed to connect to websocket server", slog.Any("error", err))
					continue
				}
				readyOnce.Do(func() { close(ready) })
				logger.Info("Connected to websocket server", slog.Any("url", y.ws.Url))
				// looping to listen
				listenCtx, cancel := context.WithCancel(ctx)
				for msg := range y.ws.Listen(listenCtx) {
					for _, handler := range y.handlers {
						if handler(msg) {
							break
						}
					}
				}
				// If Listen returns, the connection is broken, so reconnect
				logger.Warn("Websocket connection lost, attempting to reconnect")
				cancel()
				y.ws.Close()
			}
		}
	}()
	<-ready
}

func (y *Yunzai) Stop() {
	y.ws.Stop()
}

func (y *Yunzai) Send(msg Request) error {
	logger.Info("Sending message", slog.Any("message", msg))
	return y.ws.Send(msg)
}
