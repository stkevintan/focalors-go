package wechat

import (
	"flag"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/shlex"
)

type SendMessage interface {
	GetUri() string
	IsEmpty() bool
}

func (w *WechatClient) SendMessage(message SendMessage) (*ApiResult, error) {
	if message.IsEmpty() {
		return nil, fmt.Errorf("message cannot be empty")
	}
	res := &ApiResult{}
	if _, err := w.doPostAPICall(message.GetUri(), message, res); err != nil {
		return nil, err
	}
	return res, nil
}

// ============ Text Message ============
type TextMessageItem struct {
	ToUserName  string   // 接收者 wxid
	TextContent string   // 文本类型消息时内容
	MsgType     int      //1 Text 2 Image ...
	AtWxIDList  []string // 发送艾特消息时的 wxid 列表
}

type TextMessageModel struct {
	MsgItem []TextMessageItem // 消息体数组
}

func (m *TextMessageModel) GetUri() string {
	return "/message/SendTextMessage"
}

func (m *TextMessageModel) IsEmpty() bool {
	return len(m.MsgItem) == 0
}

// ============ Image Message ============
type ImageMessageItem struct {
	ToUserName   string // 接收者 wxid
	ImageContent string // 图片类型消息时图片的 base64 编码
}

type ImageMessageModel struct {
	MsgItem []ImageMessageItem // 消息体数组
}

func (m *ImageMessageModel) GetUri() string {
	return "/message/SendImageNewMessage"
}

func (m *ImageMessageModel) IsEmpty() bool {
	return len(m.MsgItem) == 0
}

// ============ Emoji Message ============
type SendEmojiItem struct {
	ToUserName string
	EmojiMd5   string
	EmojiSize  int32
}

type SendEmojiMessageModel struct {
	EmojiList []SendEmojiItem
}

func (m *SendEmojiMessageModel) GetUri() string {
	return "/message/SendEmojiMessage"
}

func (m *SendEmojiMessageModel) IsEmpty() bool {
	return len(m.EmojiList) == 0
}

// ============ App Message ============
type AppMessageItem struct {
	ToUserName  string
	ContentXML  string
	ContentType uint32 // 2001:(红包消息)
}

type AppMessageModel struct {
	AppList []AppMessageItem
}

func (m *AppMessageModel) GetUri() string {
	return "/message/SendAppMessage"
}

func (m *AppMessageModel) IsEmpty() bool {
	return len(m.AppList) == 0
}

// ====== Upload video message ======
type VideoMessageItem struct {
	ToUserName string
	VideoData  []byte
	ThumbData  string // base64
}

func (m *VideoMessageItem) GetUri() string {
	return "/message/CdnUploadVideo"
}
func (m *VideoMessageItem) IsEmpty() bool {
	return len(m.VideoData) == 0
}

// ==== Public API ====
type MessageUnit struct {
	Target  string
	Content []string
}

type WechatTarget interface {
	GetTarget() string
}

func NewMessageUnit(target WechatTarget, content ...string) *MessageUnit {
	return &MessageUnit{
		Target:  target.GetTarget(),
		Content: content,
	}
}

func NewMessageUnit2(target string, content ...string) *MessageUnit {
	return &MessageUnit{
		Target:  target,
		Content: content,
	}
}

func (w *WechatClient) SendTextBatch(messages ...*MessageUnit) (*ApiResult, error) {
	flattenedContent := make([]TextMessageItem, 0, len(messages))
	for _, m := range messages {
		for _, content := range m.Content {
			c := strings.Trim(content, " \n")
			if c == "" {
				continue
			}
			flattenedContent = append(flattenedContent, TextMessageItem{
				ToUserName:  m.Target,
				TextContent: c,
				MsgType:     1,
			})
		}
	}
	return w.SendMessage(&TextMessageModel{MsgItem: flattenedContent})
}

func (w *WechatClient) SendText(target WechatTarget, message ...string) (*ApiResult, error) {
	return w.SendTextBatch(NewMessageUnit(target, message...))
}

func (w *WechatClient) SendImageBatch(messages ...*MessageUnit) (*ApiResult, error) {
	flattenedContent := make([]ImageMessageItem, 0, len(messages))
	for _, m := range messages {
		for _, content := range m.Content {
			c := strings.TrimPrefix(content, "base64://")
			c = strings.Trim(c, " \n")
			if c == "" {
				continue
			}
			flattenedContent = append(flattenedContent, ImageMessageItem{
				ToUserName:   m.Target,
				ImageContent: c,
			})
		}
	}
	return w.SendMessage(&ImageMessageModel{MsgItem: flattenedContent})
}

func (w *WechatClient) SendImage(target WechatTarget, message ...string) (*ApiResult, error) {
	return w.SendImageBatch(NewMessageUnit(target, message...))
}

// ==== Received Message =======
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

func (w *WechatMessage) GetTarget() string {
	if w.ChatType == ChatTypeGroup {
		return w.FromGroupId
	}
	return w.FromUserId
}

func (w *WechatMessage) IsGroup() bool {
	return w.ChatType == ChatTypeGroup
}

func (w *WechatMessage) IsPrivate() bool {
	return w.ChatType == ChatTypePrivate
}

func (w *WechatMessage) IsText() bool {
	return w.MsgType == TextMessage
}

type MessageFlagSet struct {
	*flag.FlagSet
	argStr string
}

func (m *MessageFlagSet) SplitParse() error {
	args, err := shlex.Split(m.argStr)
	if err != nil {
		return err
	}
	return m.FlagSet.Parse(args)
}

func (m *MessageFlagSet) Parse() string {
	if err := m.SplitParse(); err != nil {
		if err.Error() == "flag: help requested" {
			var usageBuf strings.Builder
			m.SetOutput(&usageBuf)
			m.Usage()
			return usageBuf.String()
		}
		logger.Error("failed to parse command", slog.Any("error", err))
		return fmt.Sprintf("解析失败，发送`#%s -h` 获得帮助", m.Name())
	}
	return ""
}

func (m *WechatMessage) ToFlagSet(name string) *MessageFlagSet {
	if m.MsgType != TextMessage {
		return nil
	}
	content := strings.Trim(m.Content, " \n")
	cmdPrefix := fmt.Sprintf("#%s", name)
	if !strings.HasPrefix(content, cmdPrefix) {
		return nil
	}
	if len(content) != len(cmdPrefix) && content[len(cmdPrefix)] != ' ' {
		return nil
	}
	return &MessageFlagSet{
		FlagSet: flag.NewFlagSet(name, flag.ContinueOnError),
		argStr:  content[len(cmdPrefix):],
	}
}
