package client

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/shlex"
)

type MessageFlagSet struct {
	*flag.FlagSet
	ArgStr string
}

func (m *MessageFlagSet) SplitParse() error {
	args, err := shlex.Split(m.ArgStr)
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

type MessageUnit struct {
	Target  string
	Content []string
}

func NewMessageUnit(target SendTarget, content ...string) *MessageUnit {
	return &MessageUnit{
		Target:  target.GetTarget(),
		Content: content,
	}
}

type GenericMessage interface {
	GetId() string
	GetText() string
	// unparsed content, if any
	GetContent() string

	GetUserId() string
	GetGroupId() string
	// Target can be group id if it's a group message, or user id if it's a private message
	GetTarget() string
	GetChatType() string
	ToFlagSet(name string) *MessageFlagSet
	IsGroup() bool
	// Is a text message, as opposed to image, video, etc. Only text messages can be parsed into flag sets
	IsText() bool
	GetReferMessage() (referMsg GenericMessage, ok bool)
}

type Contact interface {
	Username() string
	Nickname() string
	AvatarUrl() string
}

type ContactGroup interface {
	Users() []Contact
	Rooms() []Contact
}

type GenericClient interface {
	Start(ctx context.Context) error
	// handler返回true表示消息已被处理，不需要继续传递给其他handler
	AddMessageHandler(handler func(ctx context.Context, msg GenericMessage) bool)
	// 发送文本消息, msg用于指定发送目标和引用消息等
	SendText(msg SendTarget, text ...string) error
	SendImage(msg SendTarget, content ...string) error
	// 获取用户或群的基本信息, 包括昵称、头像等
	GetContactDetail(userId ...string) ([]Contact, error)
	// GetGeneralContactDetails(userId ...string) (ContactGroup, error)
	GetSelfUserId() string
}
