package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"focalors-go/contract"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

const (
	// Chat type constants
	chatTypeGroup = "group"
	chatTypeP2P   = "p2p"
)

// botOpenId stores the bot's open_id, set at startup
var botOpenId string

// LarkMessage implements contract.GenericMessage
type LarkMessage struct {
	messageId        string
	msgType          string
	chatId           string
	chatType         string // p2p or group
	content          string // raw JSON content
	text             string // parsed text content
	senderId         string // open_id of the sender
	senderType       string
	mentionText      string   // text with mentions resolved
	mentionedUserIds []string // list of mentioned user open_ids
	mentionedUsers   []contract.UserInfo
	replyToMessageId string // stored for lazy resolution
	client           *LarkClient
	referOnce        sync.Once
	referMessage     contract.GenericMessage
}

var _ contract.GenericMessage = (*LarkMessage)(nil)

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

	// Extract mentioned user IDs from Mentions array
	if msg.Mentions != nil {
		lm.mentionedUserIds = make([]string, 0, len(msg.Mentions))
		lm.mentionedUsers = make([]contract.UserInfo, 0, len(msg.Mentions))
		for _, mention := range msg.Mentions {
			mentionUser := contract.UserInfo{}
			if mention.Id != nil && mention.Id.OpenId != nil {
				mentionUser.UserId = *mention.Id.OpenId
				lm.mentionedUserIds = append(lm.mentionedUserIds, mentionUser.UserId)
			}
			if mention.Name != nil {
				mentionUser.Username = *mention.Name
			}
			if mentionUser.UserId != "" || mentionUser.Username != "" {
				lm.mentionedUsers = append(lm.mentionedUsers, mentionUser)
			}
		}
	}

	var replyToMessageId string
	if msg.ParentId != nil && *msg.ParentId != "" {
		replyToMessageId = *msg.ParentId
	} else if msg.RootId != nil && *msg.RootId != "" {
		replyToMessageId = *msg.RootId
	}
	lm.replyToMessageId = replyToMessageId
	lm.client = l

	lm.text = l.extractText(lm.msgType, lm.content)

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
	if m.chatType == chatTypeGroup {
		return m.chatId
	}
	return ""
}

func (m *LarkMessage) GetTarget() string {
	return m.chatId
}

func (m *LarkMessage) IsGroup() bool {
	return m.chatType == chatTypeGroup
}

func (m *LarkMessage) IsText() bool {
	return m.msgType == "text"
}

func (m *LarkMessage) IsImage() bool {
	return m.msgType == "image"
}

func (m *LarkMessage) GetReferMessage() (contract.GenericMessage, bool) {
	if m.replyToMessageId == "" {
		return nil, false
	}
	m.referOnce.Do(func() {
		referMsg, err := m.client.getMessageByID(m.replyToMessageId)
		if err != nil {
			logger.Warn("failed to fetch referred message", slog.String("message_id", m.replyToMessageId), slog.Any("error", err))
			return
		}
		m.referMessage = referMsg
	})
	if m.referMessage == nil {
		return nil, false
	}
	return m.referMessage, true
}

func (m *LarkMessage) GetMentionedUsers() []contract.UserInfo {
	if len(m.mentionedUsers) == 0 {
		return nil
	}
	mentionedUsers := make([]contract.UserInfo, len(m.mentionedUsers))
	copy(mentionedUsers, m.mentionedUsers)
	return mentionedUsers
}

func (l *LarkClient) getMessageByID(messageId string) (*LarkMessage, error) {
	if messageId == "" {
		return nil, nil
	}

	req := larkim.NewGetMessageReqBuilder().
		MessageId(messageId).
		Build()

	resp, err := l.sdk.Im.V1.Message.Get(context.Background(), req)
	if err != nil {
		return nil, err
	}
	if !resp.Success() {
		return nil, fmt.Errorf("failed to get message: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || len(resp.Data.Items) == 0 || resp.Data.Items[0] == nil {
		return nil, nil
	}

	item := resp.Data.Items[0]
	lm := &LarkMessage{
		messageId: derefStr(item.MessageId),
		msgType:   derefStr(item.MsgType),
		chatId:    derefStr(item.ChatId),
		content:   "",
	}

	if item.Sender != nil {
		lm.senderId = derefStr(item.Sender.Id)
		lm.senderType = derefStr(item.Sender.SenderType)
		// Message.Get API returns app_id (cli_xxx) for bot-sent messages,
		// normalize to open_id so it matches GetSelfUserId()
		if lm.senderType == "app" && botOpenId != "" {
			lm.senderId = botOpenId
		}
	}
	if item.Body != nil {
		lm.content = derefStr(item.Body.Content)
	}

	if item.Mentions != nil {
		lm.mentionedUserIds = make([]string, 0, len(item.Mentions))
		lm.mentionedUsers = make([]contract.UserInfo, 0, len(item.Mentions))
		for _, mention := range item.Mentions {
			if mention == nil {
				continue
			}
			mentionUser := contract.UserInfo{}
			if mention.Id != nil {
				mentionUser.UserId = *mention.Id
				lm.mentionedUserIds = append(lm.mentionedUserIds, mentionUser.UserId)
			}
			if mention.Name != nil {
				mentionUser.Username = *mention.Name
			}
			if mentionUser.UserId != "" || mentionUser.Username != "" {
				lm.mentionedUsers = append(lm.mentionedUsers, mentionUser)
			}
		}
	}

	// Store parent ID for lazy resolution
	var parentId string
	if item.ParentId != nil && *item.ParentId != "" {
		parentId = *item.ParentId
	} else if item.RootId != nil && *item.RootId != "" {
		parentId = *item.RootId
	}
	lm.replyToMessageId = parentId
	lm.client = l

	lm.text = l.extractText(lm.msgType, lm.content)

	return lm, nil
}

// extractText parses the text from message content based on message type.
func (l *LarkClient) extractText(msgType, content string) string {
	if content == "" {
		return ""
	}
	switch msgType {
	case "text":
		var textContent struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(content), &textContent); err == nil {
			return strings.TrimSpace(mentionRegex.ReplaceAllString(textContent.Text, ""))
		}
	case "interactive":
		// Body.Content from the API may be JSON-escaped (string within string), try unescaping first
		raw := content
		if len(raw) > 0 && raw[0] == '"' {
			var unescaped string
			if err := json.Unmarshal([]byte(raw), &unescaped); err == nil {
				raw = unescaped
			}
		}
		// Lark API returns card elements as 2D array with "tag":"text"/"text":"..."
		// while sent cards use flat array with "tag":"markdown"/"content":"..."
		type cardElement struct {
			Tag     string `json:"tag"`
			Text    string `json:"text"`
			Content string `json:"content"`
		}
		var card struct {
			Elements json.RawMessage `json:"elements"`
		}
		if err := json.Unmarshal([]byte(raw), &card); err != nil {
			logger.Debug("failed to parse interactive card", slog.String("content_preview", raw[:min(len(raw), 200)]), slog.Any("error", err))
			return ""
		}
		var elems []cardElement
		// Try 2D array first (API response format), then flat array (sent format)
		var nested [][]cardElement
		if err := json.Unmarshal(card.Elements, &nested); err == nil {
			for _, row := range nested {
				elems = append(elems, row...)
			}
		} else {
			json.Unmarshal(card.Elements, &elems)
		}
		var parts []string
		for _, elem := range elems {
			text := elem.Content
			if text == "" {
				text = elem.Text
			}
			if text != "" && (elem.Tag == "markdown" || elem.Tag == "text" || elem.Tag == "div") {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	return ""
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
