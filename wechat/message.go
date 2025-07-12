package wechat

import (
	"flag"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/shlex"

	"github.com/antchfx/xmlquery"
)

type SendMessage interface {
	GetUri() string
	IsEmpty() bool
}

func (w *WechatClient) SendMessage(message SendMessage) error {
	if message.IsEmpty() {
		return fmt.Errorf("message cannot be empty")
	}
	w.sendChan <- message
	return nil
}

// ============ Text Message ============
type TextMessageItem struct {
	ToUserName  string   // 接收者 wxid
	TextContent string   // 文本类型消息时内容
	MsgType     int      //1 Text 2 Image 49 Reply...
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

func (w *WechatClient) SendTextBatch(messages ...*MessageUnit) error {
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

type WechatTargetImpl struct {
	Target string
}

func (w *WechatTargetImpl) GetTarget() string {
	return w.Target
}

func NewTarget(target string) WechatTarget {
	return &WechatTargetImpl{Target: target}
}

func (w *WechatClient) SendText(target WechatTarget, message ...string) error {
	return w.SendTextBatch(NewMessageUnit(target, message...))
}

func (w *WechatClient) SendImageBatch(messages ...*MessageUnit) error {
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

func (w *WechatClient) SendImage(target WechatTarget, message ...string) error {
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

type MessageType uint32

// const (
// 	// MMAddMsgTypeText 消息类型：文本消息
// 	MMAddMsgTypeText uint32 = 1
// 	// MMAddMsgTypeImage 消息类型：图片消息
// 	MMAddMsgTypeImage uint32 = 3
// 	// MMAddMsgTypeCard 消息类型：名片
// 	MMAddMsgTypeCard uint32 = 42
// 	//MMAddMsgTypeMov 视频消息
// 	MMAddMsgTypeMov uint32 = 47

// 	// MMAddMsgTypeRefer 消息类型：引用
// 	//MMAddMsgTypePic表情消息
// 	MMAddMsgTypePic uint32 = 47

// 	MMAddMsgTypeRefer uint32 = 49
// 	//MMAddMsgTypeVoice 语音 视频
// 	MMAddMsgTypeVoice uint32 = 50

// 	// MMAddMsgTypeStatusNotify 消息类型：状态通知
// 	MMAddMsgTypeStatusNotify uint32 = 51
// 	// MMAddMsgTypeRevokemMsg 消息类型：撤回消息
// 	MMAddMsgTypeRevokemMsg uint32 = 10002
// 	//系统消息
// 	MMAddMsgTypeSystemMsg = 10000
// )

const (
	// 消息类型：文本消息
	TextMessage MessageType = 1
	// 消息类型：图片消息
	ImageMessage MessageType = 3
	//消息类型：名片
	CardMessage MessageType = 42
	//消息类型：表情
	EmojiMessage MessageType = 47
	// 消息类型：视频
	MovMessage MessageType = 47
	// 消息类型：引用
	ReferMessage MessageType = 49
	// 消息类型：语音 视频
	VoiceMessage MessageType = 50
	// 消息类型：状态通知
	StatusNotifyMessage MessageType = 51
	// 消息类型：撤回消息
	RevokemMsgMessage MessageType = 10002
	// 系统消息
	SystemMsgMessage MessageType = 10000
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
	SelfUserId  string      `json:"self_user_id"`
	MsgId       string      `json:"msg_id"`
	MsgType     MessageType `json:"msg_type"`
	Timestamp   int64       `json:"timestamp"`
	FromUserId  string      `json:"from_user_id"`
	ToUserId    string      `json:"to_user_id"`
	FromGroupId string      `json:"from_group_id"`
	ChatType    ChatType    `json:"chat_type"`
	Content     string      `json:"content"`
	IsSelfMsg   bool        `json:"is_self_msg"`
	IsHistory   bool        `json:"is_history"`
	CreateTime  int64       `json:"create_time"`
	Text        string      `json:"text"`
	// cache keys, no need to serialize
	xml *xmlquery.Node `json:"-"`
}

/*
### 字段說明

| 字段 | 類型 | 說明 |
|------|------|------|
| key | string | 微信帳號唯一標識 |
| msgId | string | 訊息唯一ID |
| timestamp | int64 | 訊息推送時間戳 |
| fromUser | string | 發送者ID |
| toUser | string | 接收者ID |
| msgType | int | 訊息類型 |
| content | object | 訊息內容，根據訊息類型不同而不同 |
| isSelfMsg | boolean | 是否為自己發送的訊息 |
| createTime | int64 | 訊息創建時間戳 |
| isHistory | boolean | 是否為歷史訊息 |

### 訊息類型說明

| msgType | 說明 | content 格式 |
|---------|------|--------------|
| 1 | 文字訊息 | 字符串 |
| 3 | 圖片訊息 | 圖片URL或Base64 |
| 34 | 語音訊息 | 語音URL或Base64 |
| 43 | 視頻訊息 | 視頻URL或Base64 |
| 47 | 表情訊息 | 表情URL或Base64 |
| 49 | 鏈接訊息 | 包含標題、描述、URL等的對象 |
| 10000 | 系統通知 | 字符串 |
| 10002 | 撤回訊息 | 字符串 |
*/
type WechatWebHookMessage struct {
	Key        string      `json:"key"`
	MsgId      string      `json:"msgid"`
	Timestamp  int64       `json:"timestamp"`
	FromUser   string      `json:"fromuser"`
	ToUser     string      `json:"touser"`
	MsgType    MessageType `json:"msgtype"`
	Content    any         `json:"content"`
	IsSelfMsg  bool        `json:"isSelfMsg"`
	CreateTime int64       `json:"createTime"`
	IsHistory  bool        `json:"isHistory"`
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
	return w.Text != ""
}

func (w *WechatMessage) IsCommand() bool {
	return w.IsText() && strings.HasPrefix(w.Text, "#")
}

func (msg *WechatWebHookMessage) Parse() *WechatMessage {
	// map WechatWebHookMessage to WechatMessage
	message := &WechatMessage{
		MsgId:      msg.MsgId,
		SelfUserId: msg.Key,
		MsgType:    msg.MsgType,
		IsSelfMsg:  msg.IsSelfMsg,
		IsHistory:  msg.IsHistory,
		Timestamp:  msg.Timestamp,
		CreateTime: msg.CreateTime,
		FromUserId: msg.FromUser,
		ToUserId:   msg.ToUser,
		Content:    fmt.Sprintf("%v", msg.Content),
	}

	if strings.HasSuffix(message.FromUserId, "@chatroom") {
		message.ChatType = ChatTypeGroup
	} else {
		message.ChatType = ChatTypePrivate
	}

	if message.ChatType == ChatTypeGroup {
		groupId := message.FromUserId
		splited := strings.SplitN(message.Content, ":\n", 2)
		if len(splited) == 2 {
			message.FromGroupId = groupId
			message.FromUserId = splited[0]
			message.Content = splited[1]
		} else {
			logger.Warn("Failed to split group message", slog.String("Content", message.Content))
		}
	}
	if strings.HasPrefix(message.Content, "\u003c?xml") {
		// deserialize xml content
		message.Content = deserializeToXMLStr(message.Content)
		// parse xml
		message.xml, _ = xmlquery.Parse(strings.NewReader(message.Content))
	}

	switch message.MsgType {
	case TextMessage:
		message.Text = message.Content
	case ReferMessage:
		if message.xml != nil {
			title := xmlquery.FindOne(message.xml, "//appmsg/title")
			if title != nil {
				message.Text = title.InnerText()
			}
		}
	}
	return message
}

func (msg *WechatSyncMessage) Parse() WechatMessage {
	// map WechatSyncMessage to WechatMessage
	message := WechatMessage{
		MsgId:      strconv.FormatInt(msg.MsgId, 10),
		MsgType:    msg.MsgType,
		Timestamp:  time.Now().Unix(),
		CreateTime: msg.CreateTime,
		FromUserId: msg.FromUserId.Str,
		ToUserId:   msg.ToUserId.Str,
		Content:    msg.Content.Str,
	}

	if strings.HasSuffix(message.FromUserId, "@chatroom") {
		message.ChatType = ChatTypeGroup
	} else {
		message.ChatType = ChatTypePrivate
	}

	if message.ChatType == ChatTypeGroup {
		groupId := message.FromUserId
		splited := strings.SplitN(message.Content, ":\n", 2)
		if len(splited) == 2 {
			message.FromGroupId = groupId
			message.FromUserId = splited[0]
			message.Content = splited[1]
		} else {
			logger.Warn("Failed to split group message", slog.String("Content", message.Content))
		}
	}
	if strings.HasPrefix(message.Content, "\u003c?xml") {
		// deserialize xml content
		message.Content = deserializeToXMLStr(message.Content)
		// parse xml
		message.xml, _ = xmlquery.Parse(strings.NewReader(message.Content))
	}

	switch msg.MsgType {
	case TextMessage:
		message.Text = message.Content
	case ReferMessage:
		if message.xml != nil {
			title := xmlquery.FindOne(message.xml, "//appmsg/title")
			if title != nil {
				message.Text = title.InnerText()
			}
		}
	}
	return message
}

func deserializeToXMLStr(content string) string {
	// // Handle other Unicode escapes
	for strings.Contains(content, "\\u") {
		start := strings.Index(content, "\\u")
		if start == -1 || start+6 > len(content) {
			break
		}

		hexStr := content[start+2 : start+6]
		if code, err := strconv.ParseInt(hexStr, 16, 32); err == nil {
			content = content[:start] + string(rune(code)) + content[start+6:]
		} else {
			break
		}
	}

	return content
}

func (w *WechatMessage) GetReferMessage() *WechatMessage {
	if w.xml == nil {
		return nil
	}
	refer := xmlquery.FindOne(w.xml, "/msg/appmsg/refermsg")
	if refer == nil {
		return nil
	}

	typeStr := InnerText(refer, "/type")
	msgType := TextMessage
	if typeStr != "" {
		a, _ := strconv.Atoi(typeStr)
		msgType = MessageType(a)
	}
	msgIdStr := InnerText(refer, "/svrid")
	msgId := int64(0)
	if msgIdStr != "" {
		msgId, _ = strconv.ParseInt(msgIdStr, 10, 64)
	}

	fromUser := InnerText(refer, "/fromusr")
	chatUser := InnerText(refer, "/chatusr")

	if chatUser != "" {
		tmp := fromUser
		fromUser = chatUser
		chatUser = tmp
	}
	if chatUser == fromUser {
		chatUser = ""
	}

	referredMessage := &WechatMessage{
		MsgId:       fmt.Sprintf("%d", msgId),
		MsgType:     msgType,
		Timestamp:   time.Now().Unix(),
		CreateTime:  w.CreateTime,
		SelfUserId:  w.SelfUserId,
		IsSelfMsg:   w.SelfUserId == fromUser,
		FromUserId:  fromUser,
		ToUserId:    w.ToUserId,
		FromGroupId: chatUser,
		ChatType:    w.ChatType,
		Content:     InnerText(refer, "/content"),
	}
	if referredMessage.MsgType == TextMessage {
		referredMessage.Text = referredMessage.Content
	}

	if referredMessage.MsgType == ReferMessage {
		logger.Info("Refer message", slog.Any("refer", referredMessage.Content))
		referredMessage.xml, _ = xmlquery.Parse(strings.NewReader(referredMessage.Content))
		referredMessage.Text = InnerText(referredMessage.xml, "/msg/appmsg/title")
	}

	return referredMessage
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

func (m *MessageFlagSet) Rest() string {
	return strings.TrimSpace(strings.Join(m.Args(), " "))
}

func (m *WechatMessage) ToFlagSet(name string) *MessageFlagSet {
	if m.Text == "" {
		return nil
	}
	content := strings.Trim(m.Text, " \n")
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

func InnerText(refer *xmlquery.Node, xpath string) string {
	node := xmlquery.FindOne(refer, xpath)
	if node != nil {
		return node.InnerText()
	}
	return ""
}
