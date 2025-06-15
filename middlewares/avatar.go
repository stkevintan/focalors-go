package middlewares

import (
	"focalors-go/wechat"
	"log/slog"
	"regexp"
)

var triggers = regexp.MustCompile(`^[#*%]更新(面板|头像)`)

func (m *Middlewares) AddAvatarCache() {
	m.w.AddMessageHandler(func(msg *wechat.WechatMessage) bool {
		if msg.MsgType == wechat.TextMessage && triggers.MatchString(msg.Content) {
			res, err := m.w.GetContactDetails([]string{
				msg.FromUserId,
			}, []string{})
			if err != nil {
				logger.Error("Failed to get contact details", slog.Any("error", err))
				return false
			}
			for _, contact := range res.Data.ContactList {
				headUrl := contact.SmallHeadImgUrl
				key := "avatar:" + contact.UserName.Str
				m.avatarCache[key] = headUrl
				m.redis.Set(m.ctx, key, headUrl, 0)
			}
			m.w.SendText(msg, "头像已更新")
		}
		return false
	})
}
