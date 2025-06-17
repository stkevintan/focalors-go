package middlewares

import (
	"context"
	"focalors-go/config"
	"focalors-go/slogger"
	"focalors-go/wechat"
	"focalors-go/yunzai"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

var logger = slogger.New("middlewares")

type Middlewares struct {
	ctx         context.Context
	cfg         *config.Config
	w           *wechat.WechatClient
	y           *yunzai.YunzaiClient
	redis       *redis.Client
	avatarCache map[string]string
	// cron
	cron      *cron.Cron
	cronJobs  map[string]cron.EntryID
	cronMutex sync.Mutex
}

func NewMiddlewares(ctx context.Context, cfg *config.Config, w *wechat.WechatClient, y *yunzai.YunzaiClient, redis *redis.Client) *Middlewares {
	return &Middlewares{
		ctx:         ctx,
		cfg:         cfg,
		w:           w,
		y:           y,
		redis:       redis,
		cron:        cron.New(),
		avatarCache: map[string]string{},
	}
}

func (m *Middlewares) Stop() {
	m.cron.Stop()
}

func (m *Middlewares) Start() {
	m.AddLogMsg()
	m.AddJiadan()
	m.AddBridge()
	m.AddAvatarCache()
	m.cron.Start()
}

var CronJobKey = "cron:jobs"

func (m *Middlewares) AddCronJob(name string, spec string, job func()) error {
	m.cronMutex.Lock()
	defer m.cronMutex.Unlock()
	if id, exists := m.cronJobs[name]; exists {
		// remove existing job
		m.cron.Remove(id)
	}
	id, err := m.cron.AddFunc(spec, job)
	if err != nil {
		return err
	}
	m.cronJobs[name] = id
	m.redis.HSet(m.ctx, CronJobKey, name, spec)
	return nil
}

func (m *Middlewares) RemoveCronJob(name string) {
	m.cronMutex.Lock()
	defer m.cronMutex.Unlock()
	if id, exists := m.cronJobs[name]; exists {
		m.cron.Remove(id)
		delete(m.cronJobs, name)
		m.redis.HDel(m.ctx, CronJobKey, name)
	}
}

func (m *Middlewares) GetCronJobs() map[string]string {
	m.cronMutex.Lock()
	defer m.cronMutex.Unlock()
	return m.redis.HGetAll(m.ctx, CronJobKey).Val()
}
