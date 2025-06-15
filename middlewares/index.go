package middlewares

import (
	"context"
	"focalors-go/slogger"
	"focalors-go/wechat"
	"focalors-go/yunzai"

	"github.com/redis/go-redis/v9"
	"resty.dev/v3"
)

var logger = slogger.New("middlewares")

type Middlewares struct {
	ctx         context.Context
	w           *wechat.WechatClient
	y           *yunzai.YunzaiClient
	redis       *redis.Client
	avatarCache map[string]string
	client      *resty.Client
}

func NewMiddlewares(ctx context.Context, w *wechat.WechatClient, y *yunzai.YunzaiClient, redis *redis.Client) *Middlewares {
	return &Middlewares{
		ctx:         ctx,
		w:           w,
		y:           y,
		redis:       redis,
		client:      resty.New(),
		avatarCache: map[string]string{},
	}
}

func (m *Middlewares) Init() {
	m.AddLogMsg()
	m.AddJiadan()
	m.AddBridge()
	m.AddAvatarCache()
}
