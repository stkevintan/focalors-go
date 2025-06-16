package middlewares

import (
	"encoding/base64"
	"fmt"
	"focalors-go/wechat"
	"log/slog"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

func (m *Middlewares) AddJiadan() {
	var triggers = regexp.MustCompile(`^#煎蛋`)
	var JiaDanSyncKey = "jiadan:auto"
	var topN = regexp.MustCompile(`top\s*(\d+)`)
	m.w.AddMessageHandler(func(msg *wechat.WechatMessage) bool {
		if msg.MsgType == wechat.TextMessage && triggers.MatchString(msg.Content) {
			top := 1
			topR := topN.FindStringSubmatch(msg.Content)
			if len(topR) > 1 {
				parsedTop, err := strconv.Atoi(topR[1])
				if err != nil {
					logger.Warn("Failed to parse top", slog.String("top", topR[1]), slog.Any("error", err))
				} else if parsedTop > 5 {
					m.w.SendText(msg, "top 数字不能超过 5")
					return true
				} else {
					top = parsedTop
				}
			}
			urls, err := m.getJiadanTop(getKey(msg.GetTarget()), top, 0)
			if err != nil {
				logger.Error("Failed to get Jiadan URLs", slog.Any("error", err))
				m.w.SendText(msg, "获取煎蛋失败")
				return true
			}
			if len(urls) == 0 {
				m.w.SendText(msg, "没有找到新的煎蛋无聊图")
				return true
			}

			if base64Images, err := m.fetchJiadan(urls); err != nil {
				logger.Error("Failed to fetch Jiadan images", slog.Any("error", err))
				m.w.SendText(msg, "煎蛋无聊图下载失败")
			} else if len(base64Images) > 0 {
				m.w.SendImage(msg, base64Images...)
			} else {
				m.w.SendText(msg, "煎蛋无聊图下载失败")
			}
			return true
		}
		return false
	})

	jiadanSyncManager := NewJiadanSyncManager()

	startSync := func(target string) {
		c, err := jiadanSyncManager.New(target, m.cfg.Jiadan.SyncCron)
		if err != nil {
			logger.Error("Failed to create cron job", slog.String("target", target), slog.Any("error", err))
			return
		}

		_, err = c.AddFunc(m.cfg.Jiadan.SyncCron, func() {
			urls, err := m.getJiadanUpdate(getKey(target))
			if err != nil || len(urls) == 0 {
				logger.Debug("No jiadan update", slog.Any("error", err))
				return
			}
			if base64Images, err := m.fetchJiadan(urls); err != nil {
				logger.Error("Failed to fetch Jiadan images", slog.Any("error", err))
			} else if len(base64Images) > 0 {
				m.w.SendImage(wechat.NewTarget(target), base64Images...)
			}
		})

		if err != nil {
			logger.Error("Failed to add cron job", slog.String("target", target), slog.String("cron", m.cfg.Jiadan.SyncCron), slog.Any("error", err))
			jiadanSyncManager.Cancel(target)
			return
		}

		c.Start()
		logger.Info("Started jiadan auto forwarding", slog.String("target", target), slog.String("cron", m.cfg.Jiadan.SyncCron))
	}

	m.w.AddMessageHandler(func(msg *wechat.WechatMessage) bool {
		if msg.MsgType != wechat.TextMessage || msg.FromUserId != m.cfg.App.Admin {
			return false
		}
		if strings.HasPrefix(msg.Content, "#开启煎蛋定时转发") {
			key := msg.GetTarget()
			startSync(key)
			m.redis.SAdd(m.ctx, JiaDanSyncKey, key)
			m.w.SendText(msg, "煎蛋定时转发已经开启")
			return true
		}
		if strings.HasPrefix(msg.Content, "#关闭煎蛋定时转发") {
			key := msg.GetTarget()
			jiadanSyncManager.Cancel(key)
			m.redis.SRem(m.ctx, JiaDanSyncKey, key)
			m.w.SendText(msg, "煎蛋定时转发已经关闭")
			return true
		}
		return false
	})

	// automatically start jiadan forwarding
	if targets := m.redis.SMembers(m.ctx, JiaDanSyncKey).Val(); len(targets) > 0 {
		for _, target := range targets {
			startSync(target)
		}
	}
}

type JiadanSyncManager struct {
	mp map[string]*cron.Cron
	mu sync.Mutex
}

func NewJiadanSyncManager() *JiadanSyncManager {
	return &JiadanSyncManager{
		mp: make(map[string]*cron.Cron),
	}
}

func (j *JiadanSyncManager) New(target string, cronExpr string) (*cron.Cron, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	_, exists := j.mp[target]
	if exists {
		return nil, fmt.Errorf("cron job already exists for target: %s", target)
	}

	c := cron.New()
	j.mp[target] = c
	return c, nil
}

func (j *JiadanSyncManager) Cancel(target string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if c, ok := j.mp[target]; ok {
		c.Stop()
		delete(j.mp, target)
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

func (m *Middlewares) getJiadanUpdate(key string) ([]string, error) {
	commentUrl := "https://i.jandan.net/?oxwlxojflwblxbsapi=jandan.get_pic_comments"
	jiadan := &JiadanResponse{}
	urls := []string{}
	resp, err := m.client.R().SetResult(jiadan).Get(commentUrl)
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
		commentKey := fmt.Sprintf("%s%s", key, comment.CommentId)
		if m.redis.Exists(m.ctx, commentKey).Val() == 1 {
			logger.Debug("Comment already visited", slog.String("id", comment.CommentId))
			return nil, fmt.Errorf("comment already visited: %s", comment.CommentId)
		}
		for _, pic := range comment.Pics {
			if strings.HasSuffix(pic, ".gif") {
				// gif is not supported
				continue
			}
			url := useCDN(pic)
			urls = append(urls, url)
		}
		m.saveVisited(commentKey, comment)
		// break if we have too many pics
		if len(urls) >= m.cfg.Jiadan.MaxSyncCount {
			break
		}
	}
	return urls, nil
}

func (m *Middlewares) saveVisited(commentKey string, comment JiadanComment) {
	parsedTime, err := time.Parse("2006-01-02 15:04:05", comment.CommentDate)
	if err != nil {
		logger.Warn("Failed to parse time", slog.String("time", comment.CommentDate), slog.Any("error", err))
		parsedTime = time.Now()
	}
	// set key with expired after 15 days of parsedTime
	m.redis.Set(m.ctx, commentKey, strings.Join(comment.Pics, ","), time.Until(parsedTime.AddDate(0, 0, 15)))
}

func (m *Middlewares) getJiadanTop(key string, top int, page int) ([]string, error) {
	commentUrl := "https://i.jandan.net/?oxwlxojflwblxbsapi=jandan.get_pic_comments"
	if page > 0 {
		commentUrl = fmt.Sprintf("%s&page=%d", commentUrl, page)
	}

	cnt := 0
	jiadan := &JiadanResponse{}
	urls := []string{}
	resp, err := m.client.R().SetResult(jiadan).Get(commentUrl)
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
		commentKey := fmt.Sprintf("%s%s", key, comment.CommentId)
		if m.redis.Exists(m.ctx, commentKey).Val() == 1 {
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
		m.saveVisited(commentKey, comment)
		if cnt >= top {
			break
		}
	}
	if cnt < top && jiadan.CurrentPage < jiadan.PageCount {
		nextUrls, err := m.getJiadanTop(key, top-cnt, jiadan.CurrentPage+1)
		if err != nil {
			return urls, err
		}
		urls = append(urls, nextUrls...)
	}
	return urls, nil
}

func (m *Middlewares) fetchJiadan(urls []string) ([]string, error) {
	base64Images := []string{}
	for _, url := range urls {
		logger.Debug("Downloading image", slog.String("url", url))
		resp, err := m.client.R().Get(url)
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
