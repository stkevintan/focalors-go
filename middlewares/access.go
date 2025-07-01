package middlewares

import (
	"context"
	"fmt"
	"focalors-go/service"
	"focalors-go/wechat"
	"strings"
)

type AccessMiddleware struct {
	*middlewareBase
}

func NewAccessMiddleware(base *middlewareBase) Middleware {
	return &AccessMiddleware{
		middlewareBase: base,
	}
}

func (a *AccessMiddleware) OnMessage(ctx context.Context, msg *wechat.WechatMessage) bool {
	if !a.access.IsAdmin(wechat.NewTarget(msg.FromUserId)) {
		return false
	}

	if fs := msg.ToFlagSet("perm"); fs != nil {
		var kind string
		var target string
		fs.StringVar(&kind, "p", "", "权限类型")
		fs.StringVar(&target, "t", "", "目标用户wxid, 默认当前群")
		if help := fs.Parse(); help != "" {
			a.SendText(msg, help)
			return true
		}

		if kind == "" {
			a.SendText(msg, "请指定权限类型")
			return true
		}

		permType := service.NewPerm(kind)
		if permType == 0 {
			a.SendText(msg, "未知权限类型")
			return true
		}
		if target == "" && msg.IsGroup() {
			target = msg.FromGroupId
		}
		if target == "" {
			a.SendText(msg, "请指定目标用户")
			return true
		}
		nickname := target
		if !strings.HasPrefix(target, "wxid_") && !strings.HasSuffix(target, "@chatroom") {
			a.SendText(msg, fmt.Sprintf("未知目标用户: %s", target))
			return true
		}
		contact, err := a.GetGeneralContactDetails(target)
		if err == nil {
			if len(contact.Users) > 0 {
				nickname = contact.Users[0].NickName.Str
			} else if len(contact.Rooms) > 0 {
				nickname = contact.Rooms[0].NickName.Str
			}
		}

		verb := fs.Rest()

		switch verb {
		case "add":
			if err := a.access.AddPerm(wechat.NewTarget(target), permType); err != nil {
				a.SendText(msg, fmt.Sprintf("%s: 添加权限失败: %s", nickname, err.Error()))
			} else {
				a.SendText(msg, fmt.Sprintf("%s: 添加权限成功", nickname))
			}
			return true
		case "del":
			if err := a.access.RemovePerm(wechat.NewTarget(target), permType); err != nil {
				a.SendText(msg, fmt.Sprintf("%s: 删除权限失败: %s", nickname, err.Error()))
			} else {
				a.SendText(msg, fmt.Sprintf("%s: 删除权限成功", nickname))
			}
			return true
		default:
			a.SendText(msg, "未知操作")
			return true
		}
	}

	return false
}
