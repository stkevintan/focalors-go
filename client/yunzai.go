package client

import (
	"context"
	cfg "focalors-go/config"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Yunzai struct {
	ws       *WebSocketClient
	handlers []func(msg Message) bool
}

func NewYunzai(cfg *cfg.YunzaiConfig) *Yunzai {
	return &Yunzai{
		ws: New(cfg.Server),
	}
}

func (y *Yunzai) AddMessageHandler(handler func(msg Message) bool) {
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
			default:
				// Try to connect if not connected
				err := y.ws.Connect()
				if err != nil {
					logger.Error("Failed to connect to websocket server", slog.Any("error", err))
					time.Sleep(2 * time.Second)
					continue
				}
				readyOnce.Do(func() { close(ready) })
				logger.Info("Connected to websocket server", slog.Any("url", y.ws.Url))
				y.sendPing()
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
				time.Sleep(2 * time.Second)
			}
		}
	}()
	<-ready
}

func (y *Yunzai) Stop() {
	y.ws.Stop()
}

func (y *Yunzai) Send(msg any) {
	y.ws.Send(msg)
}

func (y *Yunzai) sendPing() {
	msg := map[string]any{
		"id":          uuid.NewString(),
		"type":        "meta",
		"time":        time.Now().UnixMilli(),
		"detail_type": "connect",
		"sub_type":    "",
		"self":        y.ws.Url, // or another field if you have a self identifier
		"version":     BotVersionConstant,
	}

	statusMsg := map[string]any{
		"id":          uuid.NewString(),
		"type":        "meta",
		"time":        time.Now().UnixMilli(),
		"sub_type":    "",
		"detail_type": "status_update",
		"status": map[string]any{
			"good": true,
			"bots": BotStatusConstant,
		},
	}
	y.Send(msg)
	y.Send(statusMsg)
	logger.Info("Sent ping message")
}
