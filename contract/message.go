package contract

import (
	"flag"
	"fmt"
	"focalors-go/slogger"
	"log/slog"
	"strings"

	"github.com/google/shlex"
)

var contractLogger = slogger.New("contract")

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

type UserInfo struct {
	UserId   string
	Username string
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
	IsMentioned() bool
	// get the list of mentioned users in the message, if any
	GetMentionedUsers() []UserInfo
}

func IsMentionedMe(msg GenericMessage, selfUserId string) bool {
	if msg.IsGroup() {
		for _, user := range msg.GetMentionedUsers() {
			if user.UserId == selfUserId {
				return true
			}
		}
		return false
	}
	return true
}

func IsReplyToMe(msg GenericMessage, selfUserId string) bool {
	if msg.IsGroup() {
		referMsg, ok := msg.GetReferMessage()
		if !ok {
			return false
		}
		return referMsg.GetUserId() == selfUserId
	}
	return true
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
