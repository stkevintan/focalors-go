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
	eventHandler := dispatcher.NewEventDispatcher("", l.cfg.VerificationToken).
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			// Deduplicate messages using Redis
			msgId := ""
			if event.Event != nil && event.Event.Message != nil && event.Event.Message.MessageId != nil {
				msgId = *event.Event.Message.MessageId
			}
			if msgId != "" {
				key := msgDedupeKeyPrefix + msgId
				// Try to set the key with NX (only if not exists)
				set, err := l.redis.RedisClient.SetNX(ctx, key, "1", msgDedupeTTL).Result()
				if err != nil {
					logger.Error("failed to check message dedup in redis", slog.Any("error", err))
				} else if !set {
					// Key already exists, this is a duplicate
					logger.Debug("skipping duplicate message", slog.String("messageId", msgId))
					return nil
				}
			}

			msg, err := l.parseMessage(event)
			if err != nil {
				logger.Error("failed to parse lark message", slog.Any("error", err))
				return nil
			}

			for _, handler := range l.handlers {
				if handler(ctx, msg) {
					return nil
				}
			}
			return nil
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

func (l *LarkClient) SendText(target client.SendTarget, text ...string) error {
	content := strings.Join(text, "\n")
	contentJSON, _ := json.Marshal(map[string]string{"text": content})

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(target.GetTarget()).
			MsgType(larkim.MsgTypeText).
			Content(string(contentJSON)).
			Build()).
		Build()

	resp, err := l.sdk.Im.V1.Message.Create(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to send text message: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("failed to send text message: code=%d, msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (l *LarkClient) SendImage(target client.SendTarget, content ...string) error {
	for _, c := range content {
		c = strings.TrimPrefix(c, "base64://")
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}

		imageKey, err := l.uploadBase64Image(c)
		if err != nil {
			logger.Error("failed to upload image", slog.Any("error", err))
			return fmt.Errorf("failed to upload image: %w", err)
		}

		contentJSON, _ := json.Marshal(map[string]string{"image_key": imageKey})

		req := larkim.NewCreateMessageReqBuilder().
			ReceiveIdType(larkim.ReceiveIdTypeChatId).
			Body(larkim.NewCreateMessageReqBodyBuilder().
				ReceiveId(target.GetTarget()).
				MsgType(larkim.MsgTypeImage).
				Content(string(contentJSON)).
				Build()).
			Build()

		resp, err := l.sdk.Im.V1.Message.Create(context.Background(), req)
		if err != nil {
			return fmt.Errorf("failed to send image message: %w", err)
		}
		if !resp.Success() {
			return fmt.Errorf("failed to send image message: code=%d, msg=%s", resp.Code, resp.Msg)
		}
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
	// Lark doesn't have a simple batch contact API like WeChat.
	// For now, return empty contacts. Can be extended with contact.v3 API later.
	logger.Warn("GetContactDetail is not fully implemented for Lark")
	return nil, nil
}

func (l *LarkClient) GetSelfUserId() string {
	return l.cfg.AppID
}
