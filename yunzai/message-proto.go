package yunzai

// https://docs.sayu-bot.com/CodeAdapter/Protocol.html#%E4%B8%8A%E6%8A%A5%E6%B6%88%E6%81%AF
// Request sent by the client
type Request struct {
	MsgId    string `json:"msg_id"`
	UserType string `json:"user_type"` // group/direct/channel/sub_channel
	GroupId  string `json:"group_id,omitempty"`
	// BotId     string `json:"bot_id"`
	BotSelfId string `json:"bot_self_id,omitempty"`
	// TargetType string `json:"target_type"` // direct
	// TargetId   string           `json:"target_id"`   // user_id or group_id
	UserId  string           `json:"user_id"`
	UserPM  int              `json:"user_pm"` // Permission
	Content []MessageContent `json:"content"`
	Sender  map[string]any   `json:"sender"`
}

// https://docs.sayu-bot.com/CodeAdapter/Protocol.html#%E5%8F%91%E9%80%81%E6%B6%88%E6%81%AF
// Message received by the client
type Response struct {
	// BotId      string           `json:"bot_id"`
	BotSelfId  string           `json:"bot_self_id"`
	MsgId      string           `json:"msg_id,omitempty"`
	TargetType string           `json:"target_type"` // direct
	TargetId   string           `json:"target_id"`   // user_id or group_id
	Content    []MessageContent `json:"content"`
}

// https://docs.sayu-bot.com/CodeAdapter/Protocol.html#%E6%B6%88%E6%81%AF%E7%B1%BB%E5%9E%8B-message
type MessageContent struct {
	Type string `json:"type"` // text, markdown, image, reply, at, image_size,
	Data any    `json:"data"`
}
