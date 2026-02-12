package lark

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"focalors-go/client"
	"focalors-go/config"
	"focalors-go/db"
	"focalors-go/slogger"
	"log/slog"
	"strings"
	"time"

	larkSDK "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

var logger = slogger.New("lark")

const (
	// Redis key prefix for message deduplication
	msgDedupeKeyPrefix = "lark:msg:dedup:"
	// TTL for deduplication keys
	msgDedupeTTL = 5 * time.Minute
)

type LarkClient struct {
	sdk      *larkSDK.Client
	cfg      *config.LarkConfig
	handlers []func(ctx context.Context, msg client.GenericMessage) bool
	botId    string // open_id of the bot
	redis    *db.Redis
	appCtx   context.Context // application context for graceful shutdown
}

var _ client.GenericClient = (*LarkClient)(nil)

func NewLarkClient(cfg *config.Config, redis *db.Redis) (*LarkClient, error) {
	if cfg.Lark.AppID == "" || cfg.Lark.AppSecret == "" {
		return nil, fmt.Errorf("lark appId and appSecret are required")
	}

	sdkClient := larkSDK.NewClient(cfg.Lark.AppID, cfg.Lark.AppSecret,
		larkSDK.WithEnableTokenCache(true),
	)

	return &LarkClient{
		sdk:   sdkClient,
		cfg:   &cfg.Lark,
		redis: redis,
	}, nil
}

func (l *LarkClient) Start(ctx context.Context) error {
	l.appCtx = ctx // store for use in async handlers
	eventHandler := dispatcher.NewEventDispatcher("", l.cfg.VerificationToken).
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			// Process everything asynchronously to respond to Lark immediately.
			// Lark requires response within 3 seconds, otherwise it will retry.
			go func() {
				// Deduplicate messages using Redis
				msgId := ""
				if event.Event != nil && event.Event.Message != nil && event.Event.Message.MessageId != nil {
					msgId = *event.Event.Message.MessageId
				}
				if msgId != "" {
					key := msgDedupeKeyPrefix + msgId
					// Use Background context to ensure dedup check completes regardless of event context timeout
					set, err := l.redis.RedisClient.SetNX(context.Background(), key, "1", msgDedupeTTL).Result()
					if err != nil {
						logger.Error("failed to check message dedup in redis", slog.Any("error", err))
						// On Redis error, still skip to avoid duplicate processing if this is a retry
						return
					}
					if !set {
						// Key already exists, this is a duplicate
						logger.Debug("skipping duplicate message", slog.String("messageId", msgId))
						return
					}
				}

				msg, err := l.parseMessage(event)
				if err != nil {
					logger.Error("failed to parse lark message", slog.Any("error", err))
					return
				}

				for _, handler := range l.handlers {
					if handler(l.appCtx, msg) {
						return
					}
				}
			}()
			return nil // Respond immediately to Lark
		})

	cli := larkws.NewClient(l.cfg.AppID, l.cfg.AppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
	)

	logger.Info("Starting Lark bot via WebSocket")
	return cli.Start(ctx)
}

func (l *LarkClient) AddMessageHandler(handler func(ctx context.Context, msg client.GenericMessage) bool) {
	l.handlers = append(l.handlers, handler)
}

func (l *LarkClient) RecallMessage(messageId string) error {
	if messageId == "" {
		return nil
	}

	req := larkim.NewDeleteMessageReqBuilder().
		MessageId(messageId).
		Build()

	resp, err := l.sdk.Im.V1.Message.Delete(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to recall message: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("failed to recall message: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (l *LarkClient) UploadImage(base64Content string) (string, error) {
	c := strings.TrimPrefix(base64Content, "base64://")
	c = strings.TrimSpace(c)
	if c == "" {
		return "", nil
	}
	return l.uploadBase64Image(c)
}

// buildRichCardContent creates a Lark interactive card from CardBuilder
func (l *LarkClient) buildRichCardContent(card *client.CardBuilder) string {
	elements := []map[string]interface{}{}

	for _, elem := range card.Elements {
		switch elem.Type {
		case client.CardElementMarkdown:
			elements = append(elements, map[string]interface{}{
				"tag":     "markdown",
				"content": elem.Content,
			})
		case client.CardElementImage:
			altText := elem.AltText
			if altText == "" {
				altText = "image"
			}
			elements = append(elements, map[string]interface{}{
				"tag":          "img",
				"img_key":      elem.Content,
				"custom_width": 300, // limit image width to 300px
				"alt": map[string]interface{}{
					"tag":     "plain_text",
					"content": altText,
				},
			})
		case client.CardElementDivider:
			elements = append(elements, map[string]interface{}{
				"tag": "hr",
			})
		case client.CardElementButtons:
			// Render buttons as markdown links: [`text`](data)
			var links []string
			for _, row := range elem.Buttons {
				for _, btn := range row {
					links = append(links, fmt.Sprintf("[%s](%s)", btn.Text, btn.Data))
				}
			}
			if len(links) > 0 {
				elements = append(elements, map[string]interface{}{
					"tag":     "markdown",
					"content": strings.Join(links, " "),
				})
			}
		}
	}

	cardData := map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": false,
			"update_multi":     true,
		},
		"elements": elements,
	}

	// Add header if set
	if card.Header != "" {
		cardData["header"] = map[string]interface{}{
			"title": map[string]interface{}{
				"tag":     "plain_text",
				"content": card.Header,
			},
		}
	}

	content, _ := json.Marshal(cardData)
	return string(content)
}

func (l *LarkClient) SendRichCard(target client.SendTarget, card *client.CardBuilder) (string, error) {
	return l.sendRichCardInternal(target, card)
}

func (l *LarkClient) ReplyRichCard(replyToMsgId string, target client.SendTarget, card *client.CardBuilder) (string, error) {
	if replyToMsgId == "" {
		return l.SendRichCard(target, card)
	}

	content := l.buildRichCardContent(card)

	req := larkim.NewReplyMessageReqBuilder().
		MessageId(replyToMsgId).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeInteractive).
			Content(content).
			Build()).
		Build()

	resp, err := l.sdk.Im.V1.Message.Reply(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to reply message: %w", err)
	}
	if !resp.Success() {
		return "", fmt.Errorf("failed to reply message: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data != nil && resp.Data.MessageId != nil {
		return *resp.Data.MessageId, nil
	}
	return "", nil
}

func (l *LarkClient) sendRichCardInternal(target client.SendTarget, card *client.CardBuilder) (string, error) {
	content := l.buildRichCardContent(card)

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(target.GetTarget()).
			MsgType(larkim.MsgTypeInteractive).
			Content(content).
			Build()).
		Build()

	resp, err := l.sdk.Im.V1.Message.Create(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to send rich card: %w", err)
	}
	if !resp.Success() {
		return "", fmt.Errorf("failed to send rich card: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data != nil && resp.Data.MessageId != nil {
		return *resp.Data.MessageId, nil
	}
	return "", nil
}

func (l *LarkClient) UpdateRichCard(messageId string, card *client.CardBuilder) error {
	if messageId == "" {
		return nil
	}

	content := l.buildRichCardContent(card)

	req := larkim.NewPatchMessageReqBuilder().
		MessageId(messageId).
		Body(larkim.NewPatchMessageReqBodyBuilder().
			Content(content).
			Build()).
		Build()

	resp, err := l.sdk.Im.V1.Message.Patch(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to update rich card: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("failed to update rich card: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (l *LarkClient) uploadBase64Image(b64 string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %w", err)
	}

	req := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType(larkim.ImageTypeMessage).
			Image(bytes.NewReader(data)).
			Build()).
		Build()

	resp, err := l.sdk.Im.V1.Image.Create(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}
	if !resp.Success() {
		return "", fmt.Errorf("failed to upload image: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return *resp.Data.ImageKey, nil
}

func (l *LarkClient) GetContactDetail(userId ...string) ([]client.Contact, error) {
	if len(userId) == 0 {
		return nil, nil
	}

	// Use Contact BatchGet API to get multiple users at once
	// Requires `contact:user.base:readonly` permission
	req := larkcontact.NewBatchUserReqBuilder().
		UserIds(userId).
		UserIdType("open_id").
		Build()

	resp, err := l.sdk.Contact.V3.User.Batch(context.Background(), req)
	if err != nil {
		logger.Warn("failed to batch get user info", slog.Any("error", err))
		return nil, err
	}
	if !resp.Success() {
		logger.Warn("failed to batch get user info", slog.Int("code", resp.Code), slog.String("msg", resp.Msg))
		return nil, fmt.Errorf("batch get user failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	var contacts []client.Contact
	if resp.Data != nil {
		for _, user := range resp.Data.Items {
			avatarUrl := ""
			if user.Avatar != nil && user.Avatar.AvatarOrigin != nil {
				avatarUrl = *user.Avatar.AvatarOrigin
			} else if user.Avatar != nil && user.Avatar.Avatar240 != nil {
				avatarUrl = *user.Avatar.Avatar240
			}
			contacts = append(contacts, &LarkContact{
				openId:    derefStr(user.OpenId),
				name:      derefStr(user.Name),
				avatarUrl: avatarUrl,
			})
		}
	}
	return contacts, nil
}

// LarkContact implements client.Contact
type LarkContact struct {
	openId    string
	name      string
	avatarUrl string
}

func (c *LarkContact) Username() string  { return c.openId }
func (c *LarkContact) Nickname() string  { return c.name }
func (c *LarkContact) AvatarUrl() string { return c.avatarUrl }

func (l *LarkClient) GetSelfUserId() string {
	return l.cfg.AppID
}
