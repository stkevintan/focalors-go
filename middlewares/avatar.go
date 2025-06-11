package middlewares

import (
	"focalors-go/wechat"
	"log/slog"
	"slices"
)

var triggers = []string{
	"#更新面板",
}

// Cache avatar images in redis, triggered by "#更新面板" message
func (m *Middlewares) AddAvatarCache() {
	m.w.AddMessageHandler(func(msg wechat.WechatMessage) bool {
		if msg.MsgType == wechat.TextMessage && slices.Contains(triggers, msg.Content) {
			res, err := m.w.GetContactDetails([]string{
				msg.FromUserId,
			}, []string{})
			if err != nil {
				logger.Error("Failed to get contact details", slog.Any("error", err))
				return false
			}
			for _, contact := range res.Data.ContactList {
				headUrl := contact.SmallHeadImgUrl
				m.redis.Set(m.ctx, "avatar:"+contact.UserName.Str, headUrl, 0)
			}
		}
		return false
	})
}
