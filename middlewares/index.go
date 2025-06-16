package middlewares

import (
	"context"
	"focalors-go/config"
	"focalors-go/slogger"
	"focalors-go/wechat"
	"focalors-go/yunzai"

	"github.com/redis/go-redis/v9"
)

var logger = slogger.New("middlewares")

type Middlewares struct {
	ctx         context.Context
	cfg         *config.Config
	w           *wechat.WechatClient
	y           *yunzai.YunzaiClient
	redis       *redis.Client
	avatarCache map[string]string
}

func NewMiddlewares(ctx context.Context, cfg *config.Config, w *wechat.WechatClient, y *yunzai.YunzaiClient, redis *redis.Client) *Middlewares {
	return &Middlewares{
		ctx:         ctx,
		cfg:         cfg,
		w:           w,
		y:           y,
		redis:       redis,
		avatarCache: map[string]string{},
	}
}

func (m *Middlewares) Init() {
	m.AddLogMsg()
	m.AddJiadan()
	m.AddBridge()
	m.AddAvatarCache()
}
