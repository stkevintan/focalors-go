package middlewares

import (
	"fmt"
	"focalors-go/wechat"
	"log/slog"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

func (m *Middlewares) AddAdmin() {
	IsAdmin := func(msg *wechat.WechatMessage) bool {
		return msg.FromUserId == m.cfg.App.Admin
	}

	m.w.AddMessageHandler(func(msg *wechat.WechatMessage) bool {
		if !IsAdmin(msg) {
			return false
		}
		if msg.Content == "#定时任务" && msg.IsPrivate() {
			m.cronMutex.Lock()
			entries := m.cron.Entries()
			if len(entries) == 0 {
				m.cronMutex.Unlock()
				m.w.SendText(msg, "没有定时任务")
				return true
			}

			type TaskEntry struct {
				ID   cron.EntryID
				Prev time.Time
				Next time.Time
				Wxid string
				Type string
			}

			tasks := []TaskEntry{}
			wxids := []string{}
			for _, entry := range entries {
				wxid := ""
				taskType := ""
				for key, cronId := range m.cronJobs {
					if cronId == entry.ID {
						// wxid should always be the last part of the key
						ret := strings.Split(key, ":")
						if len(ret) > 0 {
							wxid = ret[len(ret)-1]
							wxids = append(wxids, wxid)
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
			m.cronMutex.Unlock()
			var nicknameMap = make(map[string]string)
			contacts, err := m.w.GetContactDetails(wxids...)
			if err != nil {
				logger.Warn("Failed to get contact details", slog.Any("error", err))
			} else {
				for _, contact := range contacts.Users {
					nicknameMap[contact.UserName.Str] = contact.NickName.Str
				}
				for _, contact := range contacts.Rooms {
					nicknameMap[contact.UserName.Str] = contact.NickName.Str
				}
			}
			var text strings.Builder
			for _, task := range tasks {
				nickname := nicknameMap[task.Wxid]
				if nickname == "" {
					nickname = task.Wxid
				}
				text.WriteString(fmt.Sprintf("📌 任务,Type: %s |  %s(%s)\n", task.Type, nickname, task.Wxid))
				text.WriteString(fmt.Sprintf("上次执行: %s \n", task.Prev.Format("2006-01-02 15:04:05")))
				text.WriteString(fmt.Sprintf("下次执行: %s \n", task.Next.Format("2006-01-02 15:04:05")))
				text.WriteString("\n")
			}
			m.w.SendText(msg, text.String())
			return true
		}

		return false
	})
}
