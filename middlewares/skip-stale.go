package middlewares

import (
	"fmt"
	"focalors-go/wechat"
)

var maxMsgId int64 = 0

func (m *Middlewares) AddSkipStale() {
	storedMsgId, err := m.redis.Get(m.ctx, "wechat:msg_id").Int64()
	if err == nil {
		maxMsgId = storedMsgId
	}

	m.w.AddMessageHandler(func(msg *wechat.WechatMessage) bool {
		if msg.MsgId > maxMsgId {
			maxMsgId = msg.MsgId
			return false
		}
		m.w.SendText(wechat.NewTarget(m.cfg.App.Admin), fmt.Sprintf("出现消息id递减: %d -> %d", maxMsgId, msg.MsgId))
		return false
	})
}

func (m *Middlewares) AddSkipStaleOnExit() {
	m.redis.Set(m.ctx, "wechat:msg_id", maxMsgId, 0)
}
