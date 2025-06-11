package middlewares

import (
	"context"
	"focalors-go/slogger"
	"focalors-go/wechat"
	"focalors-go/yunzai"

	"github.com/redis/go-redis/v9"
)

var logger = slogger.New("middlewares")

type Middlewares struct {
	ctx   context.Context
	w     *wechat.WechatClient
	y     *yunzai.YunzaiClient
	redis *redis.Client
}

func NewMiddlewares(ctx context.Context, w *wechat.WechatClient, y *yunzai.YunzaiClient, redis *redis.Client) *Middlewares {
	return &Middlewares{
		ctx:   ctx,
		w:     w,
		y:     y,
		redis: redis,
	}
}

func (m *Middlewares) Init() {
	m.AddLogMsg()
	m.AddBridge()
	m.AddAvatarCache()
}
