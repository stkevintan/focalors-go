package wechat

import (
	"context"
	"fmt"
	"focalors-go/client"
	cfg "focalors-go/config"
	"focalors-go/slogger"
	"log/slog"
	"time"

	R "resty.dev/v3"
)

var logger = slogger.New("wechat")

type WechatClient struct {
	ws         *client.WebSocketClient[WechatSyncMessage]
	ctx        context.Context
	cfg        *cfg.WechatConfig
	httpClient *R.Client
	sendChan   chan SendMessage
	handlers   []func(ctx context.Context, msg *WechatMessage) bool
	self       *UserProfile
}

type ApiResult struct {
	Code int    `json:"Code"`
	Data any    `json:"Data"`
	Text string `json:"Text"`
}

func prettyBody(body string) string {
	threshold := 1000
	if len(body) > threshold {
		return fmt.Sprintf("%s...", body[:threshold])
	}
	return body
}

func NewWechat(ctx context.Context, cfg *cfg.Config) *WechatClient {
	httpClient := R.New()
	httpClient.
		SetBaseURL(cfg.Wechat.Server).
		SetContext(ctx).
		// SetDebug(cfg.App.Debug).
		SetTimeout(2*time.Minute).
		SetQueryParam("key", cfg.Wechat.Token).
		SetDebugLogFormatter(func(dl *R.DebugLog) string {
			req := fmt.Sprintf("\n-------------\nRequest:\nURL: %s\nHeader: %v\nBody: %s\n", dl.Request.URI, dl.Request.Header, prettyBody(dl.Request.Body))
			res := fmt.Sprintf("---------------\nResponse:\nStatus: %s\nHeader: %v\nBody: %s\n", dl.Response.Status, dl.Response.Header, prettyBody(dl.Response.Body))
			return fmt.Sprintf("%s\n%s", req, res)
		})

	return &WechatClient{
		ctx:        ctx,
		cfg:        &cfg.Wechat,
		httpClient: httpClient,
		sendChan:   make(chan SendMessage, 5),
		// ws:         client.NewClient[WechatSyncMessage](ctx, cfg.Wechat.SubURL),
	}
}

/* Login Wechat account */
func (w *WechatClient) Init() error {
	loginCtx, cancel := context.WithTimeout(w.ctx, 2*time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	loginNotify := make(chan int, 1)
	defer ticker.Stop()
	defer cancel()
	loginTimes := 0
	for {
		select {
		case <-loginCtx.Done():
			return loginCtx.Err()
		case <-ticker.C:
			status, err := w.GetLoginStatus()
			if err != nil {
				return fmt.Errorf("failed to get login status: %w", err)
			}

			if status.Data.LoginErrMsg == "账号在线状态良好！" {
				logger.Info("Account is online", slog.String("loginErrMsg", status.Data.LoginErrMsg))
				return nil
			}

			logger.Error("Failed to get login status", slog.Any("status", status), slog.Any("error", err))
			loginNotify <- loginTimes
			loginTimes++
		case loginTimes := <-loginNotify:
			if loginTimes == 0 {
				// awake login
				res, err := w.WakeUpLogin()
				logger.Info("Wake up login", slog.Any("res", res), slog.Any("error", err))
				if err == nil {
					continue
				}
			}
			// every 2 * 5 seconds
			if (loginTimes % 5) == 0 {
				// qr code login
				res2, err := w.GetLoginQRCode()
				if err != nil {
					logger.Error("Failed to get login QR code", slog.Any("error", err))
				} else {
					logger.Info("Get login QR code", slog.String("qrCodeUrl", res2.Data.QrCodeUrl))
				}
			}
		}
	}
}

func (w *WechatClient) AddMessageHandler(handler func(ctx context.Context, msg *WechatMessage) bool) {
	w.handlers = append(w.handlers, handler)
}

func (w *WechatClient) processSend() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case message := <-w.sendChan:
			res := &ApiResult{}
			if _, err := w.doPostAPICall(message.GetUri(), message, res); err != nil {
				logger.Error("Failed to send message", slog.Any("message", message), slog.Any("error", err))
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (w *WechatClient) Run() error {
	go w.processSend()
	w.self, _ = w.GetProfile()
	logger.Info("Self profile", slog.Any("self", w.self))
	switch w.cfg.PushType {
	case cfg.PushTypeWebhook:
		w.SetWebhook()
		return w.StartWebhookServer()
	case cfg.PushTypeWebSocket:
		w.ws = client.NewClient[WechatSyncMessage](w.ctx, fmt.Sprintf("%s?key=%s", w.cfg.SubURL, w.cfg.Token))
		return w.ws.Run(func(ctx context.Context, msg *WechatSyncMessage) {
			message := msg.Parse(w.self.UserInfo.UserName.Str)
			for _, handler := range w.handlers {
				if handler(ctx, message) {
					return
				}
			}
		})
	default:
		return fmt.Errorf("unsupported push type: %s", w.cfg.PushType)
	}
}

func (w *WechatClient) Dispose() {
	if w.ws != nil {
		w.ws.Close()
	}
	close(w.sendChan)
}

func (w *WechatClient) doGetAPICall(url string, res any) (*R.Response, error) {
	R := w.httpClient.R()
	resp, err := R.SetResult(res).Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("unexpected status code: %s", resp.Status())
	}

	return resp, nil
}

func (w *WechatClient) doPostAPICall(url string, body any, res any) (*R.Response, error) {
	R := w.httpClient.R()
	resp, err := R.SetResult(res).SetBody(body).Post(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("unexpected status code: %s", resp.Status())
	}

	return resp, nil
}
