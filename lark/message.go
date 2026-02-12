package lark

import (
	"encoding/json"
	"focalors-go/client"
	"regexp"
	"strings"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// LarkMessage implements client.GenericMessage
type LarkMessage struct {
	messageId   string
	msgType     string
	chatId      string
	chatType    string // p2p or group
	content     string // raw JSON content
	text        string // parsed text content
	senderId    string // open_id of the sender
	senderType  string
	mentionText string // text with mentions resolved
}

var _ client.GenericMessage = (*LarkMessage)(nil)

// mentionRegex matches Lark's @mention placeholders like @_user_1, @_all
var mentionRegex = regexp.MustCompile(`@_(?:user_\d+|all)\s*`)

func (l *LarkClient) parseMessage(event *larkim.P2MessageReceiveV1) (*LarkMessage, error) {
	msg := event.Event.Message
	sender := event.Event.Sender

	lm := &LarkMessage{
		messageId: derefStr(msg.MessageId),
		msgType:   derefStr(msg.MessageType),
		chatId:    derefStr(msg.ChatId),
		chatType:  derefStr(msg.ChatType),
		content:   derefStr(msg.Content),
	}

	if sender != nil {
		if sender.SenderId != nil {
			lm.senderId = derefStr(sender.SenderId.OpenId)
		}
		lm.senderType = derefStr(sender.SenderType)
	}

	// Parse text from content JSON for text messages
	if lm.msgType == "text" && lm.content != "" {
		var textContent struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(lm.content), &textContent); err == nil {
			// Strip @mentions from text (e.g., "@_user_1 hello" -> "hello")
			lm.text = strings.TrimSpace(mentionRegex.ReplaceAllString(textContent.Text, ""))
		}
	}

	return lm, nil
}

func (m *LarkMessage) GetId() string {
	return m.messageId
}

func (m *LarkMessage) GetText() string {
	return m.text
}

func (m *LarkMessage) GetContent() string {
	return m.content
}

func (m *LarkMessage) GetUserId() string {
	return m.senderId
}

func (m *LarkMessage) GetGroupId() string {
	if m.chatType == "group" {
		return m.chatId
	}
	return ""
}

func (m *LarkMessage) GetTarget() string {
	return m.chatId
}

func (m *LarkMessage) IsGroup() bool {
	return m.chatType == "group"
}

func (m *LarkMessage) IsText() bool {
	return m.msgType == "text"
}

func (m *LarkMessage) IsImage() bool {
	return m.msgType == "image"
}

func (m *LarkMessage) GetReferMessage() (client.GenericMessage, bool) {
	return nil, false
}

func (m *LarkMessage) IsMentioned() bool {
	return true
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
