package middlewares

import (
	"context"
	"fmt"
	"focalors-go/contract"
	"log/slog"
	"strings"
)

type adminMiddleware struct {
	*MiddlewareContext
}

func NewAdminMiddleware(base *MiddlewareContext) Middleware {
	return &adminMiddleware{
		MiddlewareContext: base,
	}
}

func (a *adminMiddleware) OnMessage(ctx context.Context, msg contract.GenericMessage) bool {
	if !a.access.IsAdmin(msg.GetUserId()) {
		return false
	}
	if fs := contract.ToFlagSet(msg, "admin"); fs != nil {
		var topic string
		fs.StringVar(&topic, "s", "", "topic: cron, access")
		if help := fs.Parse(); help != "" {
			a.SendText(msg, help)
			return true
		}
		switch topic {
		case "cron":
			return a.onCronTask(msg)
		case "access":
			return a.onAdminMessage(msg)
		default:
			a.SendText(msg, "未知主题")
			return true
		}
	}
	return false
}
func (a *adminMiddleware) onAdminMessage(msg contract.GenericMessage) bool {
	targetAndPerms, err := a.access.ListAll()
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
	if contacts, err := a.client.GetContactDetail(wxids...); err != nil {
		logger.Warn("Failed to get contact details", slog.Any("error", err))
	} else {
		for _, contact := range contacts {
			nicknameMap[contact.Username()] = contact.Nickname()
		}
	}

	for _, tp := range targetAndPerms {
		nickname := nicknameMap[tp.Target]
		if nickname == "" {
			nickname = tp.Target
		} else {
			nickname = fmt.Sprintf("%s(%s)", nickname, tp.Target)
		}
		text.WriteString(fmt.Sprintf("🔑 %s: %s\n", nickname, tp.Perm.String()))
	}

	response := text.String()
	if response == "" {
		response = "没有权限分配"
	}
	a.SendText(msg, response)
	return true
}

func (a *adminMiddleware) onCronTask(msg contract.GenericMessage) bool {
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
	if contacts, err := a.client.GetContactDetail(wxids...); err != nil {
		logger.Warn("Failed to get contact details", slog.Any("error", err))
	} else {
		for _, contact := range contacts {
			nicknameMap[contact.Username()] = contact.Nickname()
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
