package middlewares

import (
	"context"
	"encoding/base64"
	"fmt"
	"focalors-go/wechat"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"resty.dev/v3"
)

func (m *Middlewares) AddJiadan() {
	j := NewJiadanSyncManager(m.ctx, m.redis)
	m.w.AddMessageHandler(func(msg *wechat.WechatMessage) bool {
		if fs := msg.ToFlagSet("煎蛋"); fs != nil {
			var top int
			var cron string
			fs.StringVar(&cron, "c", "", "自动同步频率, cron表达式 | default (*/30 8-23 * * *) | off")
			fs.IntVar(&top, "t", 1, fmt.Sprintf("单次同步帖子数量, 1 <= N <= %d", m.cfg.Jiadan.MaxSyncCount))
			if help := fs.Parse(); help != "" {
				m.w.SendText(msg, help)
				return true
			}
			if top < 1 || top > m.cfg.Jiadan.MaxSyncCount {
				m.w.SendText(msg, fmt.Sprintf("同步帖子数量必须在1-%d之间", m.cfg.Jiadan.MaxSyncCount))
				return true
			}

			// 手动同步
			if cron == "" {
				urls, err := j.getJiadanTop(getKey(msg.GetTarget()), top, 0, false)
				if err != nil {
					logger.Error("Failed to get Jiadan URLs", slog.Any("error", err))
					m.w.SendText(msg, "获取煎蛋失败")
					return true
				}
				if len(urls) == 0 {
					m.w.SendText(msg, "没有找到新的煎蛋无聊图")
					return true
				}

				if base64Images, err := j.fetchJiadan(urls); err != nil {
					logger.Error("Failed to fetch Jiadan images", slog.Any("error", err))
					m.w.SendText(msg, "煎蛋无聊图下载失败")
				} else if len(base64Images) > 0 {
					m.w.SendImage(msg, base64Images...)
				} else {
					m.w.SendText(msg, "煎蛋无聊图下载失败")
				}
				return true
			}
			// 关闭自动同步
			if cron == "off" {
				m.RemoveCronJob(getKey(msg.GetTarget()))
				m.w.SendText(msg, "煎蛋自动同步已经关闭")
				return true
			}
			if cron == "default" {
				cron = m.cfg.Jiadan.SyncCron
			}
			if err := ValidateCronInterval(cron, 10*time.Minute); err != nil {
				m.w.SendText(msg, err.Error())
				return true
			}
			// 开启自动同步
			err := m.AddCronJob(getKey(msg.GetTarget()), j.SyncJob, map[string]string{
				"spec":   cron,
				"target": msg.GetTarget(),
				"top":    strconv.Itoa(top),
			})
			if err != nil {
				logger.Error("Failed to add cron job", slog.Any("error", err))
				m.w.SendText(msg, "煎蛋自动同步开启失败, 请检查cron表达式")
			} else {
				m.w.SendText(msg, "煎蛋自动同步已经开启")
			}
			return true
		}

		if msg.Content == "#煎蛋任务" && msg.FromUserId == m.cfg.App.Admin && msg.ChatType == wechat.ChatTypePrivate {
			entries := m.cron.Entries()
			if len(entries) == 0 {
				m.w.SendText(msg, "没有煎蛋任务")
				return true
			}
			var tasks strings.Builder
			m.cronMutex.Lock()
			for _, entry := range entries {
				name := "未知"
				for key, id := range m.cronJobs {
					if id == entry.ID {
						name = key
						break
					}
				}
				tasks.WriteString(fmt.Sprintf("任务 %s(%d): 上次执行: %s, 下次执行: %s\n", name, entry.ID, entry.Prev.String(), entry.Next.String()))
			}
			m.cronMutex.Unlock()
			m.w.SendText(msg, tasks.String())
			return true
		}
		return false
	})

	// start a goroutine to send images
	go func() {
		defer close(j.Images)
		for {
			select {
			case msg := <-j.Images:
				m.w.SendImageBatch(msg)
				time.Sleep(2 * time.Second)
			case <-m.ctx.Done():
				return
			}
		}
	}()

	// automatically start jiadan sync on startup
	if params := m.GetCronJobs(getKey("*")); len(params) > 0 {
		for _, p := range params {
			target := p["target"]
			if target == "" {
				logger.Warn("Invalid cron job params", slog.Any("params", p))
				continue
			}
			if err := m.AddCronJob(getKey(target), j.SyncJob, p); err != nil {
				logger.Error("Failed to add cron job", slog.Any("error", err))
			} else {
				logger.Info("Jiadan auto sync enabled", slog.String("target", target), slog.Any("params", p))
			}
		}
	}
}

func ValidateCronInterval(spec string, minInterval time.Duration) error {
	fields := strings.Fields(spec)
	if len(fields) < 5 {
		return fmt.Errorf("cron表达式无效")
	}
	minuteField := fields[0]
	if minuteField == "*" {
		return fmt.Errorf("分钟字段不能为*")
	}
	// Only handle step values like */N
	if strings.HasPrefix(minuteField, "*/") {
		n, err := strconv.Atoi(minuteField[2:])
		if err != nil {
			return fmt.Errorf("分钟字段无效: %v", err)
		}
		if time.Duration(n)*time.Minute < minInterval {
			return fmt.Errorf("定时任务最小间隔不能小于 %v", minInterval)
		}
	}
	return nil
}

func (j *JiadanSyncManager) SyncJob(ctx map[string]string) {
	target := ctx["target"]
	topStr := ctx["top"]
	top, _ := strconv.Atoi(topStr)
	if top <= 0 {
		top = 1
	}
	urls, err := j.getJiadanTop(getKey(target), top, 0, true)
	if err != nil || len(urls) == 0 {
		logger.Debug("No jiadan update", slog.Any("error", err), slog.String("target", target))
		return
	}
	if base64Images, err := j.fetchJiadan(urls); err != nil {
		logger.Error("Failed to fetch Jiadan images", slog.Any("error", err), slog.String("target", target))
	} else if len(base64Images) > 0 {
		j.Images <- &wechat.MessageUnit{
			Target:  target,
			Content: base64Images,
		}
	}
}

type JiadanSyncManager struct {
	client *resty.Client
	Images chan *wechat.MessageUnit

	ctx   context.Context
	redis *redis.Client
}

func NewJiadanSyncManager(ctx context.Context, redis *redis.Client) *JiadanSyncManager {
	return &JiadanSyncManager{
		client: resty.New().SetRetryCount(3).SetRetryWaitTime(1 * time.Second),
		ctx:    ctx,
		redis:  redis,
		Images: make(chan *wechat.MessageUnit, 20),
	}
}

type JiadanComment struct {
	CommentId     string `json:"comment_ID"`
	CommentAuthor string `json:"comment_author"`
	CommentDate   string `json:"comment_date"`
	// VotePositive    string   `json:"vote_positive"`
	// VoteNegative    string   `json:"vote_negative"`
	// TextContent     string   `json:"text_content"`
	// SubCommentCount string   `json:"sub_comment_count"`
	Pics []string `json:"pics"`
}

type JiadanResponse struct {
	CurrentPage int             `json:"current_page"`
	PageCount   int             `json:"page_count"`
	Comments    []JiadanComment `json:"comments"`
}

func getKey(id string) string {
	return fmt.Sprintf("jiadan:%s", id)
}

func useCDN(url string) string {
	file := path.Base(url)
	return fmt.Sprintf("https://img.toto.im/large/%s", file)
}

func (j *JiadanSyncManager) saveVisited(commentKey string, comment JiadanComment) {
	parsedTime, err := time.Parse("2006-01-02 15:04:05", comment.CommentDate)
	if err != nil {
		logger.Warn("Failed to parse time", slog.String("time", comment.CommentDate), slog.Any("error", err))
		parsedTime = time.Now()
	}
	// set key with expired after 15 days of parsedTime
	j.redis.Set(j.ctx, commentKey, strings.Join(comment.Pics, ","), time.Until(parsedTime.AddDate(0, 0, 15)))
}

func (j *JiadanSyncManager) getJiadanTop(key string, top int, page int, syncMode bool) ([]string, error) {
	commentUrl := "https://i.jandan.net/?oxwlxojflwblxbsapi=jandan.get_pic_comments"
	if page > 0 {
		commentUrl = fmt.Sprintf("%s&page=%d", commentUrl, page)
	}

	cnt := 0
	jiadan := &JiadanResponse{}
	urls := []string{}
	resp, err := j.client.R().SetResult(jiadan).Get(commentUrl)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("unexpected status code: %s", resp.Status())
	}

	for _, comment := range jiadan.Comments {
		logger.Debug("Jiadan comment", slog.String("id", comment.CommentId))
		if comment.CommentAuthor == "sein" {
			continue
		}
		commentKey := fmt.Sprintf("%s:%s", key, comment.CommentId)
		if j.redis.Exists(j.ctx, commentKey).Val() == 1 {
			if syncMode {
				return nil, fmt.Errorf("comment already visited: %s", comment.CommentId)
			}
			logger.Debug("Comment already visited", slog.String("id", comment.CommentId))
			continue
		}
		hasImage := false
		for _, pic := range comment.Pics {
			if strings.HasSuffix(pic, ".gif") {
				// gif is not supported
				continue
			}
			url := useCDN(pic)
			hasImage = true
			urls = append(urls, url)
		}

		if hasImage {
			cnt++
		}
		j.saveVisited(commentKey, comment)
		if cnt >= top {
			break
		}
	}
	if cnt < top && jiadan.CurrentPage < jiadan.PageCount {
		nextUrls, err := j.getJiadanTop(key, top-cnt, jiadan.CurrentPage+1, syncMode)
		if err != nil {
			return urls, err
		}
		urls = append(urls, nextUrls...)
	}
	return urls, nil
}

func (j *JiadanSyncManager) fetchJiadan(urls []string) ([]string, error) {
	base64Images := []string{}
	for _, url := range urls {
		logger.Debug("Downloading image", slog.String("url", url))
		resp, err := j.client.R().Get(url)
		if err != nil {
			logger.Error("Failed to download image", slog.String("url", url), slog.Any("error", err))
			continue
		}

		if !resp.IsSuccess() { // Checks for 2xx status codes
			logger.Error("Failed to download image, non-success status",
				slog.String("url", url),
				slog.String("status", resp.Status()),
				slog.String("body", resp.String()), // Log body for debugging if it's not too large
			)
			continue
		}

		// Get the raw bytes of the image
		imageBytes := resp.Bytes()

		// Convert the image bytes to a base64 string
		base64Str := base64.StdEncoding.EncodeToString(imageBytes)
		base64Images = append(base64Images, base64Str)
	}
	return base64Images, nil
}
