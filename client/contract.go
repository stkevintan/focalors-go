package client

import (
	"context"
	"flag"
	"fmt"
	"focalors-go/slogger"
	"log/slog"
	"strings"

	"github.com/google/shlex"
)

var contractLogger = slogger.New("client.contract")

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
		contractLogger.Error("failed to parse command", slog.Any("error", err))
		return fmt.Sprintf("解析失败，发送`#%s -h` 获得帮助", m.Name())
	}
	return ""
}

func (m *MessageFlagSet) Rest() string {
	return strings.TrimSpace(strings.Join(m.Args(), " "))
}

type SendTarget interface {
	GetTarget() string
}

type SendTargetImpl struct {
	Target string
}

func (w *SendTargetImpl) GetTarget() string {
	return w.Target
}

func NewTarget(target string) SendTarget {
	return &SendTargetImpl{Target: target}
}

type GenericMessage interface {
	// GetId returns the unique id of the message, if available. Otherwise returns empty string
	GetId() string
	// parsed text content, if the message is a text message and can be parsed into a flag set, otherwise empty string
	GetText() string
	// unparsed content, if any
	GetContent() string
	// GetUserId returns the user id of the sender
	GetUserId() string
	// GetGroupId returns the group id if the message is sent in a group chat, otherwise returns empty string
	GetGroupId() string
	// Target can be group id if it's a group message, or user id if it's a private message
	GetTarget() string
	// IsGroup returns true if the message is sent in a group chat
	IsGroup() bool
	// Is a text message, as opposed to image, video, etc. Only text messages can be parsed into flag sets
	IsText() bool
	// IsImage returns true if the message is an image message
	IsImage() bool
	// GetReferMessage returns the message being replied to, if any
	GetReferMessage() (referMsg GenericMessage, ok bool)
	// GetMentionedUserIds() []string
	IsMentioned() bool
}

func ToFlagSet(m GenericMessage, name string) *MessageFlagSet {
	if m.GetText() == "" {
		return nil
	}
	content := strings.Trim(m.GetText(), " \n")
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

type Contact interface {
	Username() string
	Nickname() string
	AvatarUrl() string
}

// CardElementType represents the type of card element
type CardElementType int

const (
	CardElementMarkdown CardElementType = iota
	CardElementImage
	CardElementDivider
	CardElementButtons
)

// Button represents a clickable button
type Button struct {
	Text string // button display text
	Data string // data sent when clicked
}

// CardElement represents a single element in a card
type CardElement struct {
	Type    CardElementType
	Content string     // markdown text or image key
	AltText string     // alt text for images
	Buttons [][]Button // 2D array of buttons (rows)
}

// CardBuilder helps build cards with multiple elements
type CardBuilder struct {
	Header   string // optional header title
	Elements []CardElement
}

func NewCardBuilder() *CardBuilder {
	return &CardBuilder{Elements: []CardElement{}}
}

// AddHeader sets the card header title
func (b *CardBuilder) AddHeader(title string) *CardBuilder {
	b.Header = title
	return b
}

func (b *CardBuilder) AddMarkdown(markdown string) *CardBuilder {
	b.Elements = append(b.Elements, CardElement{Type: CardElementMarkdown, Content: markdown})
	return b
}

func (b *CardBuilder) AddImage(imageKey string, altText string) *CardBuilder {
	b.Elements = append(b.Elements, CardElement{Type: CardElementImage, Content: imageKey, AltText: altText})
	return b
}

func (b *CardBuilder) AddDivider() *CardBuilder {
	b.Elements = append(b.Elements, CardElement{Type: CardElementDivider})
	return b
}

// AddButtons adds button rows to the card
func (b *CardBuilder) AddButtons(buttons [][]Button) *CardBuilder {
	b.Elements = append(b.Elements, CardElement{Type: CardElementButtons, Buttons: buttons})
	return b
}

type Sendable interface {
	// 发送富卡片消息 (多个元素: 文本/图片/分割线)
	SendRichCard(msg SendTarget, card *CardBuilder) (messageId string, err error)
}

type GenericClient interface {
	Sendable
	Start(ctx context.Context) error
	// handler返回true表示消息已被处理，不需要继续传递给其他handler
	AddMessageHandler(handler func(ctx context.Context, msg GenericMessage) bool)
	// 撤回消息
	RecallMessage(messageId string) error
	// 上传图片 (base64), 返回图片key
	UploadImage(base64Content string) (imageKey string, err error)
	// 更新富卡片消息
	UpdateRichCard(messageId string, card *CardBuilder) error
	// 回复指定消息
	ReplyRichCard(replyToMsgId string, target SendTarget, card *CardBuilder) (messageId string, err error)
	// 获取用户或群的基本信息, 包括昵称、头像等
	GetContactDetail(userId ...string) ([]Contact, error)
	GetSelfUserId() string
	// 下载消息中的图片，返回 base64 编码
	DownloadMessageImage(msgId string) (base64Content string, err error)
}

// SendText sends a simple text message using RichCard
func SendText(c GenericClient, msg SendTarget, text string) (string, error) {
	return c.SendRichCard(msg, NewCardBuilder().AddMarkdown(text))
}

// SendImage sends a simple image message using RichCard
func SendImage(c GenericClient, msg SendTarget, base64Content string) (string, error) {
	imageKey, err := c.UploadImage(base64Content)
	if err != nil {
		return "", err
	}
	return c.SendRichCard(msg, NewCardBuilder().AddImage(imageKey, "image"))
}
