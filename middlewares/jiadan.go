package middlewares

import (
	"context"
	"fmt"
	"focalors-go/client"
	"focalors-go/db"
	"focalors-go/scheduler"
	"focalors-go/service"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

type jiadanMiddleware struct {
	*MiddlewareContext
	jiadan   *service.JiadanService
	sendLock sync.Mutex
}

func NewJiadanMiddleware(base *MiddlewareContext) Middleware {
	return &jiadanMiddleware{
		MiddlewareContext: base,
		jiadan:            service.NewJiadanService(db.NewJiandanStore(base.redis)),
	}
}

func (j *jiadanMiddleware) Start() error {
	// automatically start jiadan sync on startup
	if params := j.cron.GetCronJobs(getKey("*")); len(params) > 0 {
		for _, p := range params {
			target := p["target"]
			if target == "" {
				logger.Warn("Invalid cron job params", slog.Any("params", p))
				continue
			}
			if err := j.cron.AddCronJob(getKey(target), j.SyncJob(), p); err != nil {
				logger.Error("Failed to add cron job", slog.Any("error", err))
			} else {
				logger.Info("Jiadan auto sync enabled", slog.String("target", target), slog.Any("params", p))
			}
		}
	}
	return nil
}

func (j *jiadanMiddleware) OnMessage(ctx context.Context, msg client.GenericMessage) bool {
	if fs := client.ToFlagSet(msg, "煎蛋"); fs != nil {
		var top int
		var cron string
		fs.StringVar(&cron, "c", "", "自动同步频率, cron表达式 | default (*/30 8-23 * * *) | off")
		fs.IntVar(&top, "t", 1, fmt.Sprintf("单次同步帖子数量, 1 <= N <= %d", j.cfg.Jiadan.MaxSyncCount))
		sender := j.SendPendingReply(msg)
		if help := fs.Parse(); help != "" {
			sender.SendMarkdown(help)
			return true
		}
		if top < 1 || top > j.cfg.Jiadan.MaxSyncCount {
			sender.SendMarkdown(fmt.Sprintf("同步帖子数量必须在1-%d之间", j.cfg.Jiadan.MaxSyncCount))
			return true
		}

		// 手动同步
		if cron == "" {
			posts, err := j.jiadan.FetchNewImages(msg.GetTarget(), top)
			if err != nil {
				logger.Error("Failed to get Jiadan images", slog.Any("error", err))
				sender.SendMarkdown("获取煎蛋失败")
				return true
			}
			if len(posts) == 0 {
				sender.SendMarkdown("没有找到新的煎蛋无聊图")
				return true
			}
			card := j.buildJiadanCard(posts)
			sender.SendRichCard(card)
			return true
		}
		// 关闭自动同步
		if cron == "off" {
			j.cron.RemoveCronJob(getKey(msg.GetTarget()))
			sender.SendMarkdown("煎蛋自动同步已经关闭")
			return true
		}
		if cron == "default" || cron == "on" || cron == "auto" {
			cron = j.cfg.App.SyncCron
		}
		if err := scheduler.ValidateCronInterval(cron, 10*time.Minute); err != nil {
			sender.SendMarkdown(err.Error())
			return true
		}
		// 开启自动同步
		err := j.cron.AddCronJob(getKey(msg.GetTarget()), j.SyncJob(), map[string]string{
			"spec":   cron,
			"target": msg.GetTarget(),
			"top":    strconv.Itoa(top),
		})
		if err != nil {
			logger.Error("Failed to add cron job", slog.Any("error", err))
			sender.SendMarkdown("煎蛋自动同步开启失败, 请检查cron表达式")
		} else {
			sender.SendMarkdown("煎蛋自动同步已经开启")
		}
		return true
	}
	return false
}

func (j *jiadanMiddleware) SyncJob() func(ctx map[string]string) error {
	return func(ctx map[string]string) error {
		target := ctx["target"]
		topStr := ctx["top"]
		top, _ := strconv.Atoi(topStr)
		if top <= 0 {
			top = 1
		}
		logger.Debug("Start jiadan sync job", slog.String("target", target), slog.Int("top", top))

		base64Images, err := j.jiadan.FetchNewImages(target, top)
		if err != nil {
			return fmt.Errorf("failed to get Jiadan images: %w", err)
		}
		if len(base64Images) == 0 {
			logger.Debug("No new Jiadan images", slog.String("target", target))
			return nil
		}

		j.sendLock.Lock()
		defer j.sendLock.Unlock()
		card := j.buildJiadanCard(base64Images)
		if _, err := j.client.SendRichCard(client.NewTarget(target), card); err != nil {
			logger.Error("Failed to send jiadan card", slog.Any("error", err))
		}
		return nil
	}
}

// buildJiadanCard creates a card with all jiadan images uploaded
func (j *jiadanMiddleware) buildJiadanCard(posts []service.JiadanResult) *client.CardBuilder {
	totalImages := 0
	for _, p := range posts {
		totalImages += len(p.Images)
	}
	card := client.NewCardBuilder().AddMarkdown(fmt.Sprintf("**煎蛋无聊图** (%d张)", totalImages))
	for _, post := range posts {
		card.AddMarkdown(fmt.Sprintf("%s (%s) 👍%s 👎%s",
			post.CommentAuthor, post.CommentDate, post.VotePositive, post.VoteNegative))
		for _, img := range post.Images {
			if img == "" {
				continue
			}
			imageKey, err := j.client.UploadImage(img)
			if err != nil {
				logger.Error("Failed to upload jiadan image", slog.Any("error", err))
				continue
			}
			if imageKey == "" {
				continue
			}
			card.AddImage(imageKey, "煎蛋无聊图")
		}
	}
	return card
}

func getKey(id string) string {
	return fmt.Sprintf("jiadan:%s", id)
}
