package middlewares

import (
	"encoding/base64"
	"fmt"
	"focalors-go/wechat"
	"log/slog"
	"path"
	"strconv"
	"strings"
	"time"

	"resty.dev/v3"
)

type JiadanMiddleware struct {
	*MiddlewareBase
	client *resty.Client
	Images chan *wechat.MessageUnit
	cron   *CronUtil
}

func NewJiadanMiddleware(base *MiddlewareBase, cron *CronUtil) *JiadanMiddleware {
	return &JiadanMiddleware{
		MiddlewareBase: base,
		client:         resty.New().SetRetryCount(3).SetRetryWaitTime(1 * time.Second),
		Images:         make(chan *wechat.MessageUnit, 20),
		cron:           cron,
	}
}

func (j *JiadanMiddleware) Start() {
	j.MiddlewareBase.OnStart()
	// start a goroutine to send images
	go func() {
		for msg := range j.Images {
			j.w.SendImageBatch(msg)
			time.Sleep(2 * time.Second) // 控制发送频率，避免过快
		}
	}()

	// automatically start jiadan sync on startup
	if params := j.cron.GetCronJobs(getKey("*")); len(params) > 0 {
		for _, p := range params {
			target := p["target"]
			if target == "" {
				logger.Warn("Invalid cron job params", slog.Any("params", p))
				continue
			}
			if err := j.cron.AddCronJob(getKey(target), j.SyncJob, p); err != nil {
				logger.Error("Failed to add cron job", slog.Any("error", err))
			} else {
				logger.Info("Jiadan auto sync enabled", slog.String("target", target), slog.Any("params", p))
			}
		}
	}
}

func (j *JiadanMiddleware) Stop() {
	j.MiddlewareBase.OnStop()
	close(j.Images)
}

func (j *JiadanMiddleware) OnWechatMessage(msg *wechat.WechatMessage) bool {
	if fs := msg.ToFlagSet("煎蛋"); fs != nil {
		var top int
		var cron string
		fs.StringVar(&cron, "c", "", "自动同步频率, cron表达式 | default (*/30 8-23 * * *) | off")
		fs.IntVar(&top, "t", 1, fmt.Sprintf("单次同步帖子数量, 1 <= N <= %d", j.cfg.Jiadan.MaxSyncCount))
		if help := fs.Parse(); help != "" {
			j.w.SendText(msg, help)
			return true
		}
		if top < 1 || top > j.cfg.Jiadan.MaxSyncCount {
			j.w.SendText(msg, fmt.Sprintf("同步帖子数量必须在1-%d之间", j.cfg.Jiadan.MaxSyncCount))
			return true
		}

		// 手动同步
		if cron == "" {
			urls, err := j.getJiadanTop(getKey(msg.GetTarget()), top, 0, false)
			if err != nil {
				logger.Error("Failed to get Jiadan URLs", slog.Any("error", err))
				j.w.SendText(msg, "获取煎蛋失败")
				return true
			}
			if len(urls) == 0 {
				j.w.SendText(msg, "没有找到新的煎蛋无聊图")
				return true
			}

			if base64Images, err := j.fetchJiadan(urls); err != nil {
				logger.Error("Failed to fetch Jiadan images", slog.Any("error", err))
				j.w.SendText(msg, "煎蛋无聊图下载失败")
			} else if len(base64Images) > 0 {
				j.w.SendImage(msg, base64Images...)
			} else {
				j.w.SendText(msg, "煎蛋无聊图下载失败")
			}
			return true
		}
		// 关闭自动同步
		if cron == "off" {
			j.cron.RemoveCronJob(getKey(msg.GetTarget()))
			j.w.SendText(msg, "煎蛋自动同步已经关闭")
			return true
		}
		if cron == "default" {
			cron = j.cfg.Jiadan.SyncCron
		}
		if err := ValidateCronInterval(cron, 10*time.Minute); err != nil {
			j.w.SendText(msg, err.Error())
			return true
		}
		// 开启自动同步
		err := j.cron.AddCronJob(getKey(msg.GetTarget()), j.SyncJob, map[string]string{
			"spec":   cron,
			"target": msg.GetTarget(),
			"top":    strconv.Itoa(top),
		})
		if err != nil {
			logger.Error("Failed to add cron job", slog.Any("error", err))
			j.w.SendText(msg, "煎蛋自动同步开启失败, 请检查cron表达式")
		} else {
			j.w.SendText(msg, "煎蛋自动同步已经开启")
		}
		return true
	}
	return false
}

func (j *JiadanMiddleware) SyncJob(ctx map[string]string) {
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

func (j *JiadanMiddleware) saveVisited(commentKey string, comment JiadanComment) {
	parsedTime, err := time.Parse("2006-01-02 15:04:05", comment.CommentDate)
	if err != nil {
		logger.Warn("Failed to parse time", slog.String("time", comment.CommentDate), slog.Any("error", err))
		parsedTime = time.Now()
	}
	// set key with expired after 15 days of parsedTime
	j.redis.Set(commentKey, strings.Join(comment.Pics, ","), time.Until(parsedTime.AddDate(0, 0, 15)))
}

func (j *JiadanMiddleware) getJiadanTop(key string, top int, page int, syncMode bool) ([]string, error) {
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
		if ok, _ := j.redis.Exists(commentKey); ok {
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

func (j *JiadanMiddleware) fetchJiadan(urls []string) ([]string, error) {
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
