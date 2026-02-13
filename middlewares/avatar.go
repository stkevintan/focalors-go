package middlewares

import (
	"context"
	"fmt"
	"focalors-go/client"
	"log/slog"
	"time"
)

const (
	avatarSessionPrefix = "avatar:session:"
	avatarSessionTTL    = 1 * time.Minute
)

type avatarMiddleware struct {
	*MiddlewareContext
}

func NewAvatarMiddleware(base *MiddlewareContext) Middleware {
	return &avatarMiddleware{
		MiddlewareContext: base,
	}
}

func (a *avatarMiddleware) OnMessage(ctx context.Context, msg client.GenericMessage) bool {
	// Check if user has an active avatar upload session (highest priority to capture images)
	if a.handleAvatarUpload(msg) {
		return true
	}

	// Check for #上传头像 command
	if msg.IsText() && msg.GetText() == "#上传头像" {
		return a.handleAvatarCommand(msg)
	}

	return false
}

func (a *avatarMiddleware) handleAvatarCommand(msg client.GenericMessage) bool {
	// Only allow in private chat
	if msg.IsGroup() {
		a.SendText(msg, "请在私聊中使用 #上传头像 功能")
		return true
	}

	userId := msg.GetUserId()
	sessionKey := avatarSessionPrefix + userId

	// Don't allow creating another session if one is already active
	if existing, _ := a.redis.Get(sessionKey); existing != "" {
		a.SendText(msg, "你已经有一个上传会话进行中，请发送图片或等待超时")
		return true
	}

	// Create a session with 1 minute timeout
	if err := a.redis.Set(sessionKey, "pending", avatarSessionTTL); err != nil {
		logger.Error("Failed to create avatar session", slog.Any("error", err))
		a.SendText(msg, "创建上传会话失败，请稍后重试")
		return true
	}

	a.SendText(msg, "请在1分钟内发送一张图片作为你的头像")

	// Start a goroutine to notify on timeout
	go func() {
		time.Sleep(avatarSessionTTL)
		// Check if session is still pending (not consumed by an upload)
		if val, _ := a.redis.Get(sessionKey); val != "" {
			a.redis.Del(sessionKey)
			a.SendText(msg, "上传头像会话已超时，请重新发送 #上传头像")
		}
	}()

	return true
}

func (a *avatarMiddleware) handleAvatarUpload(msg client.GenericMessage) bool {
	// Only handle private image messages
	if msg.IsGroup() || !msg.IsImage() {
		return false
	}

	userId := msg.GetUserId()
	sessionKey := avatarSessionPrefix + userId

	// Check if user has an active session
	val, err := a.redis.Get(sessionKey)
	if err != nil || val == "" {
		return false
	}

	// Download the image
	base64Content, err := a.client.DownloadMessageImage(msg.GetId())
	if err != nil {
		logger.Error("Failed to download avatar image",
			slog.String("userId", userId),
			slog.String("msgId", msg.GetId()),
			slog.Any("error", err),
		)
		a.SendText(msg, "下载图片失败，请重新发送图片")
		return true
	}

	// Store avatar via AvatarStore
	if err := a.avatarStore.Save(userId, base64Content); err != nil {
		logger.Error("Failed to store avatar",
			slog.String("userId", userId),
			slog.Any("error", err),
		)
		a.SendText(msg, "保存头像失败，请稍后重试")
		return true
	}

	// Clear the session
	if err := a.redis.Del(sessionKey); err != nil {
		logger.Warn("Failed to clear avatar session", slog.Any("error", err))
	}

	logger.Info("Avatar uploaded successfully",
		slog.String("userId", userId),
		slog.Int("size", len(base64Content)),
	)
	a.SendText(msg, fmt.Sprintf("头像上传成功！(大小: %d bytes)", len(base64Content)))
	return true
}

func (a *avatarMiddleware) Start() error {
	return nil
}

func (a *avatarMiddleware) Stop() error {
	return nil
}
