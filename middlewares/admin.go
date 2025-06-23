package middlewares

import (
	"fmt"
	"focalors-go/scheduler"
	"focalors-go/wechat"
	"log/slog"
	"strings"
)

type adminMiddleware struct {
	*MiddlewareBase
	cron *scheduler.CronTask
}

func newAdminMiddleware(base *MiddlewareBase, cron *scheduler.CronTask) *adminMiddleware {
	return &adminMiddleware{
		MiddlewareBase: base,
		cron:           cron,
	}
}

func (a *adminMiddleware) OnWechatMessage(msg *wechat.WechatMessage) bool {
	if !msg.IsCommand() || msg.FromUserId != a.cfg.App.Admin {
		return false
	}
	if msg.Content == "#定时任务" && msg.IsPrivate() {
		return a.onCronTask(msg)
	}
	return false
}

func (a *adminMiddleware) onCronTask(msg *wechat.WechatMessage) bool {
	tasks := a.cron.TaskEntries()
	if len(tasks) == 0 {
		a.SendText(msg, "没有定时任务")
		return true
	}
	var nicknameMap = make(map[string]string)
	wxids := make([]string, 0, len(tasks))
	for _, entry := range tasks {
		wxids = append(wxids, entry.Wxid)
	}
	contacts, err := a.GetContactDetails(wxids...)
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
	a.SendText(msg, text.String())
	return true
}
