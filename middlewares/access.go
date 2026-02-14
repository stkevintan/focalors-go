package middlewares

import (
	"context"
	"fmt"
	"focalors-go/contract"
	"focalors-go/service"
)

type AccessMiddleware struct {
	*MiddlewareContext
}

func NewAccessMiddleware(base *MiddlewareContext) Middleware {
	return &AccessMiddleware{
		MiddlewareContext: base,
	}
}

func (a *AccessMiddleware) OnMessage(ctx context.Context, msg contract.GenericMessage) bool {
	if !a.access.IsAdmin(msg.GetUserId()) {
		return false
	}

	if fs := contract.ToFlagSet(msg, "access"); fs != nil {
		var kind string
		var target string
		fs.StringVar(&kind, "p", "", "权限类型")
		fs.StringVar(&target, "u", "", "目标用户wxid, 默认当前群")
		if help := fs.Parse(); help != "" {
			a.SendText(msg, help)
			return true
		}

		if kind == "" {
			a.SendText(msg, "请指定权限类型")
			return true
		}

		permType := service.NewAccess(kind)
		if permType == 0 {
			a.SendText(msg, "未知权限类型")
			return true
		}
		if target == "" && msg.IsGroup() {
			target = msg.GetGroupId()
		}
		if target == "" {
			a.SendText(msg, "请指定目标用户")
			return true
		}
		nickname := target
		// if !strings.HasPrefix(target, "wxid_") && !strings.HasSuffix(target, "@chatroom") {
		// 	a.contract.SendText(msg, fmt.Sprintf("未知目标用户: %s", target))
		// 	return true
		// }

		if contacts, err := a.client.GetContactDetail(target); err == nil && len(contacts) > 0 {
			nickname = contacts[0].Nickname()
		}

		verb := fs.Rest()

		switch verb {
		case "add":
			if err := a.access.AddAccess(target, permType); err != nil {
				a.SendText(msg, fmt.Sprintf("%s: 添加权限失败: %s", nickname, err.Error()))
			} else {
				a.SendText(msg, fmt.Sprintf("%s: 添加权限成功", nickname))
			}
			return true
		case "del":
			if err := a.access.DelAccess(target, permType); err != nil {
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
