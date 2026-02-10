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

type GenericClient interface {
	Start(ctx context.Context) error
	// handler返回true表示消息已被处理，不需要继续传递给其他handler
	AddMessageHandler(handler func(ctx context.Context, msg GenericMessage) bool)
	// 发送文本消息, msg用于指定发送目标和引用消息等
	SendText(msg SendTarget, text ...string) error
	SendImage(msg SendTarget, content ...string) error
	// 获取用户或群的基本信息, 包括昵称、头像等
	GetContactDetail(userId ...string) ([]Contact, error)
	GetSelfUserId() string
}
