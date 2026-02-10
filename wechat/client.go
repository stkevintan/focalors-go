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
	cfg        *cfg.WechatConfig
	httpClient *R.Client
	handlers   []WechatMessageHandler
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

func NewWechat(cfg *cfg.Config) (*WechatClient, error) {
	httpClient := R.New()
	httpClient.
		SetBaseURL(cfg.Wechat.Server).
		// SetDebug(cfg.App.Debug).
		SetTimeout(2*time.Minute).
		SetQueryParam("key", cfg.Wechat.Token).
		SetDebugLogFormatter(func(dl *R.DebugLog) string {
			req := fmt.Sprintf("\n-------------\nRequest:\nURL: %s\nHeader: %v\nBody: %s\n", dl.Request.URI, dl.Request.Header, prettyBody(dl.Request.Body))
			res := fmt.Sprintf("---------------\nResponse:\nStatus: %s\nHeader: %v\nBody: %s\n", dl.Response.Status, dl.Response.Header, prettyBody(dl.Response.Body))
			return fmt.Sprintf("%s\n%s", req, res)
		})

	w := &WechatClient{
		cfg:        &cfg.Wechat,
		httpClient: httpClient,
		// ws:         client.NewClient[WechatSyncMessage](ctx, cfg.Wechat.SubURL),
	}
	return w, nil
}

/* Login Wechat account */
func (w *WechatClient) login(ctx context.Context) error {
	if w.cfg.Token == "" {
		return nil
	}
	loginCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
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

type WechatMessageHandler = func(ctx context.Context, msg client.GenericMessage) bool

func (w *WechatClient) AddMessageHandler(handler WechatMessageHandler) {
	w.handlers = append(w.handlers, handler)
}

var self *UserProfile

// func (w *WechatClient) processSend(sendChan chan SendMessage) {

// }

func (w *WechatClient) Start(ctx context.Context) error {
	w.httpClient.SetContext(ctx)
	if err := w.login(ctx); err != nil {
		return err
	}
	w.self, _ = w.GetProfile()
	self = w.self
	logger.Info("Self profile", slog.Any("self", w.self))
	switch w.cfg.PushType {
	case cfg.PushTypeWebhook:
		w.SetWebhook()
		return w.StartWebhookServer()
	case cfg.PushTypeWebSocket:
		w.ws = client.NewClient[WechatSyncMessage](fmt.Sprintf("%s?key=%s", w.cfg.SubURL, w.cfg.Token))
		return w.ws.Run(ctx, func(msg *WechatSyncMessage) {
			message := msg.Parse()
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

func (w *WechatClient) GetSelfUserId() string {
	if w.self == nil {
		panic("wechat client is not started yet")
	}
	return w.self.UserInfo.UserName.Str
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
