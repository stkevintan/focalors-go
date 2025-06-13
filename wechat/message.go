package wechat

import (
	"fmt"
	"strings"
)

type MessageItem struct {
	ToUserName   string   // 接收者 wxid
	TextContent  string   // 文本类型消息时内容
	ImageContent string   // 图片类型消息时图片的 base64 编码
	MsgType      int      //1 Text 2 Image
	AtWxIDList   []string // 发送艾特消息时的 wxid 列表
}

type SendMessageModel struct {
	MsgItem []MessageItem // 消息体数组
}

func (w *WechatClient) SendTextMessage(messages []MessageItem) (*ApiResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}
	res := &ApiResult{}
	if _, err := w.doPostAPICall("/message/SendTextMessage", SendMessageModel{MsgItem: messages}, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (w *WechatClient) SendImageMessage(messages []MessageItem) (*ApiResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}
	res := &ApiResult{}
	if _, err := w.doPostAPICall("/message/SendImageMessage", SendMessageModel{MsgItem: messages}, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (w *WechatClient) SendImageNewMessage(messages []MessageItem) (*ApiResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}
	res := &ApiResult{}
	if _, err := w.doPostAPICall("/message/SendImageNewMessage", SendMessageModel{MsgItem: messages}, res); err != nil {
		return nil, err
	}
	return res, nil
}

type SendEmojiItem struct {
	ToUserName string
	EmojiMd5   string
	EmojiSize  int32
}

type SendEmojiMessageModel struct {
	EmojiList []SendEmojiItem
}

func (w *WechatClient) SendEmojiMessage(messages []SendEmojiItem) (*ApiResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}
	res := &ApiResult{}
	if _, err := w.doPostAPICall("/message/SendEmojiMessage", SendEmojiMessageModel{EmojiList: messages}, res); err != nil {
		return nil, err
	}
	return res, nil
}

type AppMessageItem struct {
	ToUserName  string
	ContentXML  string
	ContentType uint32 // 2001:(红包消息)
}

type AppMessageModel struct {
	AppList []AppMessageItem
}

func (w *WechatClient) SendAppMessage(messages []AppMessageItem) (*ApiResult, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("messages cannot be empty")
	}
	res := &ApiResult{}
	if _, err := w.doPostAPICall("/message/SendAppMessage", AppMessageModel{AppList: messages}, res); err != nil {
		return nil, err
	}
	return res, nil
}

type WechatTarget interface {
	GetReplyTarget() string
}

func (w *WechatClient) SendTextMessageTo(target WechatTarget, content ...string) (*ApiResult, error) {
	flattenedContent := make([]MessageItem, 0, len(content))
	for _, c := range content {
		c = strings.Trim(c, " \n")
		if c == "" {
			continue
		}
		flattenedContent = append(flattenedContent, MessageItem{
			ToUserName:  target.GetReplyTarget(),
			TextContent: c,
			MsgType:     1,
		})
	}
	return w.SendTextMessage(flattenedContent)
}

type StrWrapper struct {
	Str string `json:"str"`
}

type WechatImageMessageBuf struct {
	Len    int    `json:"len"`
	Buffer string `json:"buffer"`
}

type MessageType int

const (
	TextMessage  MessageType = 1
	ImageMessage MessageType = 3
	EmojiMessage MessageType = 47
	AppMessage   MessageType = 49
)

type ChatType int

const (
	ChatTypePrivate ChatType = iota
	ChatTypeGroup   ChatType = iota
)

type WechatMessageBase struct {
	MsgId        int64                 `json:"msg_id"`
	MsgType      MessageType           `json:"msg_type"` // 1: 文本消息 3: 图片 47: emoji 49: app
	Status       int                   `json:"status"`
	ImgStatus    int                   `json:"img_status"`
	ImageBuf     WechatImageMessageBuf `json:"image_buf"`
	CreateTime   int64                 `json:"create_time"`
	MsgSource    string                `json:"msg_source"`
	PushContent  string                `json:"push_content"`
	NewMessageId int64                 `json:"new_message_id"`
}
type WechatSyncMessage struct {
	WechatMessageBase
	FromUserId StrWrapper `json:"from_user_name"`
	ToUserId   StrWrapper `json:"to_user_name"`
	Content    StrWrapper `json:"content"`
}

type WechatMessage struct {
	WechatMessageBase
	FromUserId  string   `json:"from_user_id"`
	ToUserId    string   `json:"to_user_id"`
	FromGroupId string   `json:"from_group_id"`
	ChatType    ChatType `json:"chat_type"`
	Content     string   `json:"content"`
}

func (w *WechatMessage) GetReplyTarget() string {
	if w.ChatType == ChatTypeGroup {
		return w.FromGroupId
	}
	return w.FromUserId
}
