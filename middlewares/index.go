package middlewares

import (
	"context"
	"fmt"
	"focalors-go/client"
	"focalors-go/config"
	"focalors-go/db"
	"focalors-go/scheduler"
	"focalors-go/service"
	"focalors-go/slogger"
	"log/slog"
)

var logger = slogger.New("middlewares")

type Middleware interface {
	OnMessage(ctx context.Context, msg client.GenericMessage) bool
	Start() error
	Stop() error
}

type MiddlewareContext struct {
	redis  *db.Redis
	cron   *scheduler.CronTask
	cfg    *config.Config
	access *service.AccessService
	ctx    context.Context
	client client.GenericClient
}

func NewMiddlewareContext(ctx context.Context, client client.GenericClient, cfg *config.Config, redis *db.Redis) *MiddlewareContext {
	cron := scheduler.NewCronTask(redis)
	access := service.NewAccessService(redis, cfg.App.Admin)
	// init
	cron.Start()
	return &MiddlewareContext{
		redis:  redis,
		cron:   cron,
		cfg:    cfg,
		access: access,
		ctx:    ctx,
		client: client,
	}
}

// PendingSender automatically updates/recalls the pending message before sending a new message.
// Uses card messages for in-place updates on supported platforms.
type PendingSender struct {
	client       client.GenericClient
	target       client.SendTarget
	pendingMsgId string
	replyToMsgId string // optional: message ID to reply to
}

// NewPendingSender creates a PendingSender that will recall pendingMsgId before sending
func NewPendingSender(c client.GenericClient, target client.SendTarget, pendingMsgId string) *PendingSender {
	return &PendingSender{
		client:       c,
		target:       target,
		pendingMsgId: pendingMsgId,
	}
}

// NewReplySender creates a PendingSender that replies to a specific message
func NewReplySender(c client.GenericClient, target client.SendTarget, pendingMsgId, replyToMsgId string) *PendingSender {
	return &PendingSender{
		client:       c,
		target:       target,
		pendingMsgId: pendingMsgId,
		replyToMsgId: replyToMsgId,
	}
}

func (p *PendingSender) recallPending() {
	if p.pendingMsgId != "" {
		if err := p.client.RecallMessage(p.pendingMsgId); err != nil {
			logger.Error("failed to recall pending message", slog.Any("error", err))
		}
		p.pendingMsgId = ""
	}
}

// SendRichCard updates pending card in place if possible, otherwise recalls and sends new
func (p *PendingSender) SendRichCard(card *client.CardBuilder) (string, error) {
	if p.pendingMsgId != "" {
		// Try to update the card in place
		if err := p.client.UpdateRichCard(p.pendingMsgId, card); err != nil {
			logger.Error("failed to update card, falling back to recall", slog.Any("error", err))
			p.recallPending()
			return p.sendNewCard(card)
		}
		msgId := p.pendingMsgId
		p.pendingMsgId = ""
		return msgId, nil
	}
	p.recallPending()
	return p.sendNewCard(card)
}

func (p *PendingSender) sendNewCard(card *client.CardBuilder) (string, error) {
	if p.replyToMsgId != "" {
		return p.client.ReplyRichCard(p.replyToMsgId, p.target, card)
	}
	return p.client.SendRichCard(p.target, card)
}

// SendMarkdown is a convenience method for sending a simple markdown message
func (p *PendingSender) SendMarkdown(markdown string) (string, error) {
	return p.SendRichCard(client.NewCardBuilder().AddMarkdown(markdown))
}

// UploadImage uploads an image and returns the image key
func (p *PendingSender) UploadImage(base64Content string) (string, error) {
	return p.client.UploadImage(base64Content)
}

// SendPendingMessage sends a "loading" card message and returns a PendingSender
// that will automatically update the card when SendRichCard is called
func (m *MiddlewareContext) SendPendingMessage(msg client.SendTarget) *PendingSender {
	// Send a loading card (supports in-place update)
	loadingCard := client.NewCardBuilder().AddMarkdown("少女祈祷中...")
	id, err := m.client.SendRichCard(msg, loadingCard)
	if err != nil {
		logger.Warn("failed to send loading card", slog.Any("error", err))
		return NewPendingSender(m.client, msg, "")
	}
	return NewPendingSender(m.client, msg, id)
}

// SendPendingReply sends a "loading" card as a reply to the trigger message
// and returns a PendingSender that will update the card in place
func (m *MiddlewareContext) SendPendingReply(msg client.GenericMessage) *PendingSender {
	loadingCard := client.NewCardBuilder().AddMarkdown("少女祈祷中...")
	id, err := m.client.ReplyRichCard(msg.GetId(), msg, loadingCard)
	if err != nil {
		logger.Warn("failed to send loading reply", slog.Any("error", err))
		return NewReplySender(m.client, msg, "", msg.GetId())
	}
	return NewReplySender(m.client, msg, id, msg.GetId())
}

func (mctx *MiddlewareContext) Close() {
	mctx.cron.Stop()
}

func (m *MiddlewareContext) OnMessage(ctx context.Context, msg client.GenericMessage) bool {
	return false
}

func (m *MiddlewareContext) Start() error {
	return nil
}

func (m *MiddlewareContext) Stop() error {
	return nil
}

// SendText sends a simple text message
func (m *MiddlewareContext) SendText(msg client.SendTarget, text string) (string, error) {
	return client.SendText(m.client, msg, text)
}

// SendImage sends a simple image message
func (m *MiddlewareContext) SendImage(msg client.SendTarget, base64Content string) (string, error) {
	return client.SendImage(m.client, msg, base64Content)
}

type RootMiddleware struct {
	*MiddlewareContext
	// sync lock?
	middlewares []Middleware
}

func NewRootMiddleware(
	mctx *MiddlewareContext,
) *RootMiddleware {
	return &RootMiddleware{
		MiddlewareContext: mctx,
	}
}

func (r *RootMiddleware) AddMiddlewares(middlewares ...func(m *MiddlewareContext) Middleware) {
	for _, mw := range middlewares {
		instance := mw(r.MiddlewareContext)
		if instance != nil {
			r.middlewares = append(r.middlewares, instance)
		}
	}
}

func (r *RootMiddleware) Start() error {
	for _, mw := range r.middlewares {
		if r.client != nil {
			r.client.AddMessageHandler(mw.OnMessage)
		}
		if err := mw.Start(); err != nil {
			return err
		}
		logger.Info("Middleware started", slog.String("type", fmt.Sprintf("%T", mw)))
	}
	return nil
}

func (r *RootMiddleware) Stop() error {
	for _, mw := range r.middlewares {
		if err := mw.Stop(); err != nil {
			return err
		}
	}
	return nil
}
