package yunzai

import (
	"context"
	"focalors-go/client"
	"focalors-go/config"
)

type YunzaiClient struct {
	ws       *client.WebSocketClient[Response]
	cfg      *config.Config
	handlers []func(ctx context.Context, msg *Response) bool
}

func NewYunzai(ctx context.Context, cfg *config.Config) *YunzaiClient {
	return &YunzaiClient{
		ws:  client.NewClient[Response](ctx, cfg.Yunzai.Server),
		cfg: cfg,
	}
}

func (y *YunzaiClient) AddMessageHandler(handler func(ctx context.Context, msg *Response) bool) {
	y.handlers = append(y.handlers, handler)
}

func (y *YunzaiClient) Run() error {
	return y.ws.Run(func(ctx context.Context, msg *Response) {
		for _, handler := range y.handlers {
			if handler(ctx, msg) {
				break
			}
		}
	})
}

func (y *YunzaiClient) Dispose() {
	y.ws.Close()
}

func (y *YunzaiClient) Send(message Request) error {
	return y.ws.Send(message)
}
