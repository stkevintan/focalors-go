package client

type BotSelf struct {
	Platform string `json:"platform"`
	UserId   string `json:"user_id"`
}

var BotSelfConstant = BotSelf{
	Platform: "wechat",
	UserId:   "focalors",
}

type Message struct {
	Action string         `json:"action"`
	Echo   string         `json:"echo"`
	Params map[string]any `json:"params"`
}

type BotVersion struct {
	Version       string `json:"version"`
	Impl          string `json:"impl"`
	OneBotVersion string `json:"onebot_version"`
}

var BotVersionConstant = BotVersion{
	Impl:          "ComWechat",
	Version:       "1.2.0",
	OneBotVersion: "12",
}

type BotStatus struct {
	Online bool    `json:"online"`
	Self   BotSelf `json:"self"`
}

var BotStatusConstant = []BotStatus{
	{
		Online: true,
		Self:   BotSelfConstant,
	},
}

type StatusUpdate struct {
	Good bool        `json:"good"`
	Bots []BotStatus `json:"bots"`
}
