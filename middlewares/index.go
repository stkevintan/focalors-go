package middlewares

import (
	"context"
	"fmt"
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

func getCronKey(name string) string {
	return fmt.Sprintf("cron:job:%s", name)
}

func (m *Middlewares) AddCronJob(name string, job func(params map[string]string), params map[string]string) error {
	m.cronMutex.Lock()
	defer m.cronMutex.Unlock()
	spec := params["spec"]
	if spec == "" {
		spec = m.cfg.Jiadan.SyncCron
	}
	if id, exists := m.cronJobs[name]; exists {
		// remove existing job
		m.cron.Remove(id)
	}
	id, err := m.cron.AddFunc(spec, func() {
		job(params)
	})
	if err != nil {
		return err
	}
	m.cronJobs[name] = id
	key := getCronKey(name)
	m.redis.HSet(m.ctx, key, params)
	return nil
}

func (m *Middlewares) RemoveCronJob(name string) {
	m.cronMutex.Lock()
	defer m.cronMutex.Unlock()
	if id, exists := m.cronJobs[name]; exists {
		m.cron.Remove(id)
		delete(m.cronJobs, name)
		key := getCronKey(name)
		m.redis.Del(m.ctx, key)
	}
}

func (m *Middlewares) GetCronJobs(key string) (jobs []map[string]string) {
	m.cronMutex.Lock()
	defer m.cronMutex.Unlock()

	// iterate all the keys with "cron:job:{key}"
	keys := m.redis.Keys(m.ctx, getCronKey(key)).Val()
	for _, key := range keys {
		val := m.redis.HGetAll(m.ctx, key).Val()
		jobs = append(jobs, val)
	}
	return
}
