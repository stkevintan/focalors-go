package contract

import "context"

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
