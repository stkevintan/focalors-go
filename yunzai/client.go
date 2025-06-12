package yunzai

import (
	"context"
	"focalors-go/client"
	"focalors-go/config"
)

type YunzaiClient struct {
	ws       *client.WebSocketClient[Response]
	cfg      *config.Config
	handlers []func(msg Response) bool
}

func NewYunzai(ctx context.Context, cfg *config.Config) *YunzaiClient {
	return &YunzaiClient{
		ws:  client.NewClient[Response](ctx, cfg.Yunzai.Server),
		cfg: cfg,
	}
}

func (y *YunzaiClient) AddMessageHandler(handler func(msg Response) bool) {
	y.handlers = append(y.handlers, handler)
}

func (y *YunzaiClient) syncMessage() {
	for msg := range y.ws.Message {
		for _, handler := range y.handlers {
			if handler(msg) {
				break
			}
		}
	}
}
func (y *YunzaiClient) Start() error {
	go y.syncMessage()
	return y.ws.Listen()
}

func (y *YunzaiClient) Stop() {
	y.ws.Close()
}

func (y *YunzaiClient) Send(message Request) error {
	return y.ws.Send(message)
}
