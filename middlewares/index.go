package middlewares

import (
	"context"
	"focalors-go/config"
	"focalors-go/slogger"
	"focalors-go/wechat"
	"focalors-go/yunzai"
	"time"

	"github.com/redis/go-redis/v9"
	"resty.dev/v3"
)

var logger = slogger.New("middlewares")

type Middlewares struct {
	ctx         context.Context
	cfg         *config.Config
	w           *wechat.WechatClient
	y           *yunzai.YunzaiClient
	redis       *redis.Client
	avatarCache map[string]string
	client      *resty.Client
}

func NewMiddlewares(ctx context.Context, cfg *config.Config, w *wechat.WechatClient, y *yunzai.YunzaiClient, redis *redis.Client) *Middlewares {
	return &Middlewares{
		ctx:         ctx,
		cfg:         cfg,
		w:           w,
		y:           y,
		redis:       redis,
		client:      resty.New().SetRetryCount(3).SetRetryWaitTime(1 * time.Second),
		avatarCache: map[string]string{},
	}
}

func (m *Middlewares) Init() {
	m.AddLogMsg()
	m.AddJiadan()
	m.AddBridge()
	m.AddAvatarCache()
}
