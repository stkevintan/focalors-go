package middlewares

import (
	"context"
	"fmt"
	"focalors-go/wechat"
	"log/slog"
	"strings"
)

type adminMiddleware struct {
	*middlewareBase
}

func NewAdminMiddleware(base *middlewareBase) Middleware {
	return &adminMiddleware{
		middlewareBase: base,
	}
}

func (a *adminMiddleware) OnMessage(_ctx context.Context, msg *wechat.WechatMessage) bool {
	if !a.access.IsAdmin(msg) {
		return false
	}
	if fs := msg.ToFlagSet("admin"); fs != nil {
		var topic string
		fs.StringVar(&topic, "s", "", "topic: cron, perm")
		if help := fs.Parse(); help != "" {
			a.SendText(msg, help)
			return true
		}
		switch topic {
		case "cron":
			return a.onCronTask(msg)
		case "perm":
			return a.onAdminMessage(msg)
		default:
			a.SendText(msg, "未知主题")
			return true
		}
	}
	return false
}
func (a *adminMiddleware) onAdminMessage(msg *wechat.WechatMessage) bool {
	targetAndPerms, err := a.access.ListTargetAndPerm()
	if err != nil {
		logger.Warn("Failed to list target and perm", slog.Any("error", err))
		a.SendText(msg, "获取权限列表失败")
		return true
	}
	var text strings.Builder
	text.Grow(len(targetAndPerms) * 10)

	var nicknameMap = make(map[string]string, len(targetAndPerms)) // Pre-allocate capacity
	wxids := make([]string, 0, len(targetAndPerms))
	for _, entry := range targetAndPerms {
		wxids = append(wxids, entry.Target)
	}
	contacts, err := a.GetGeneralContactDetails(wxids...)
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

	for _, tp := range targetAndPerms {
		nickname := nicknameMap[tp.Target]
		if nickname == "" {
			nickname = tp.Target
		} else {
			nickname = fmt.Sprintf("%s(%s)", nickname, tp.Target)
		}
		text.WriteString(fmt.Sprintf("🔑 %s | %s | %s\n", nickname, tp.Perm.String(), tp.Perm))
	}

	a.SendText(msg, text.String())
	return true
}

func (a *adminMiddleware) onCronTask(msg *wechat.WechatMessage) bool {
	tasks := a.cron.TaskEntries()
	if len(tasks) == 0 {
		a.SendText(msg, "没有定时任务")
		return true
	}
	var nicknameMap = make(map[string]string, len(tasks)) // Pre-allocate capacity
	wxids := make([]string, 0, len(tasks))
	for _, entry := range tasks {
		wxids = append(wxids, entry.Wxid)
	}
	contacts, err := a.GetGeneralContactDetails(wxids...)
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
	// Pre-allocate string builder with estimated capacity
	var text strings.Builder
	text.Grow(len(tasks) * 100)
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
