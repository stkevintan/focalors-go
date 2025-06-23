package middlewares

import (
	"fmt"
	"focalors-go/db"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type CronTask struct {
	cron      *cron.Cron
	cronJobs  map[string]cron.EntryID
	cronMutex sync.Mutex
	redis     *db.Redis
}

func (m *CronTask) Start() {
	m.cron.Start()
}

func (m *CronTask) Stop() {
	m.cron.Stop()
}

func newCronTask(redis *db.Redis) *CronTask {
	return &CronTask{
		cron:     cron.New(),
		cronJobs: make(map[string]cron.EntryID),
		redis:    redis,
	}
}

func getCronKey(name string) string {
	return fmt.Sprintf("cron:job:%s", name)
}

func (m *CronTask) AddCronJob(name string, job func(params map[string]string), params map[string]string) error {
	m.cronMutex.Lock()
	defer m.cronMutex.Unlock()
	spec := params["spec"]
	if spec == "" {
		return fmt.Errorf("cron job %s: spec is required", name)
	}

	id, err := m.cron.AddFunc(spec, func() {
		job(params)
	})
	if err != nil {
		return err
	}
	// delete previous job if exists
	if id, exists := m.cronJobs[name]; exists {
		// remove existing job
		m.cron.Remove(id)
	}
	m.cronJobs[name] = id
	key := getCronKey(name)
	m.redis.HSet(key, params)
	return nil
}

func (m *CronTask) RemoveCronJob(name string) {
	m.cronMutex.Lock()
	defer m.cronMutex.Unlock()
	if id, exists := m.cronJobs[name]; exists {
		m.cron.Remove(id)
		delete(m.cronJobs, name)
		key := getCronKey(name)
		m.redis.Del(key)
	}
}

func (m *CronTask) GetCronJobs(key string) (jobs []map[string]string) {
	m.cronMutex.Lock()
	defer m.cronMutex.Unlock()

	// iterate all the keys with "cron:job:{key}"
	keys, _ := m.redis.Keys(getCronKey(key))
	for _, key := range keys {
		val, err := m.redis.HGetAll(key)
		if err != nil {
			continue // skip if there's an error
		}
		jobs = append(jobs, val)
	}
	return
}

func (m *CronTask) Entries() []cron.Entry {
	m.cronMutex.Lock()
	defer m.cronMutex.Unlock()
	return m.cron.Entries()
}

type TaskEntry struct {
	ID   cron.EntryID
	Prev time.Time
	Next time.Time
	Wxid string
	Type string
}

func (m *CronTask) TaskEntries() []TaskEntry {
	m.cronMutex.Lock()
	defer m.cronMutex.Unlock()

	entries := m.cron.Entries()
	tasks := make([]TaskEntry, 0, len(entries))
	for _, entry := range entries {
		wxid := ""
		taskType := ""
		for key, cronId := range m.cronJobs {
			if cronId == entry.ID {
				ret := strings.Split(key, ":")
				if len(ret) > 0 {
					wxid = ret[len(ret)-1]
					taskType = strings.Join(ret[0:len(ret)-1], ":")
				}
				break
			}
		}
		tasks = append(tasks, TaskEntry{
			ID:   entry.ID,
			Prev: entry.Prev,
			Next: entry.Next,
			Wxid: wxid,
			Type: taskType,
		})
	}
	return tasks
}

func ValidateCronInterval(spec string, minInterval time.Duration) error {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(spec)
	if err != nil {
		return fmt.Errorf("cron表达式无效: %v", err)
	}
	now := time.Now()
	prev := schedule.Next(now)
	// Check for 1 days ahead
	end := now.Add(1 * 24 * time.Hour)
	for prev.Before(end) {
		next := schedule.Next(prev)
		if next.After(end) {
			break
		}
		interval := next.Sub(prev)
		if interval < minInterval {
			return fmt.Errorf("cron表达式间隔时间太短: %s", interval)
		}
		prev = next
	}
	return nil
}
