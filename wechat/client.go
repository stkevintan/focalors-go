package wechat

import (
	"context"
	"fmt"
	"focalors-go/client"
	cfg "focalors-go/config"
	"focalors-go/slogger"
	"log/slog"
	"strings"
	"time"

	R "resty.dev/v3"
)

var logger = slogger.New("wechat")

type WechatClient struct {
	ctx        context.Context
	cfg        *cfg.WechatConfig
	httpClient *R.Client
	ws         *client.WebSocketClient[WechatSyncMessage]
	handlers   []func(msg WechatMessage) bool
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
		SetDebug(cfg.App.Debug).
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
		ws:         client.NewClient[WechatSyncMessage](ctx, cfg.Wechat.SubURL+"?key="+cfg.Wechat.Token),
	}
}

func (w *WechatClient) initAccount() error {
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
			if err == nil {
				logger.Info("Account is online", slog.String("loginErrMsg", status.Data.LoginErrMsg))
				return nil
			}
			// fatal error
			if status == nil {
				return err
			}

			if status.Data.LoginErrMsg != "账号在线状态良好！" {
				logger.Error("Failed to get login status", slog.Any("status", status), slog.Any("error", err))
				loginNotify <- loginTimes
				loginTimes++
				continue
			}
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
					continue
				}
				logger.Info("Get login QR code", slog.String("qrCodeUrl", res2.Data.QrCodeUrl))
			}
		}
	}
}

func (w *WechatClient) AddMessageHandler(handler func(msg WechatMessage) bool) {
	w.handlers = append(w.handlers, handler)
}

func (w *WechatClient) syncMessage() {
	// map WechatSyncMessage to WechatMessage
	w.ws.SetMessageHandlers(func(msg WechatSyncMessage) bool {
		message := WechatMessage{
			WechatMessageBase: msg.WechatMessageBase,
			FromUserId:        msg.FromUserId.Str,
			ToUserId:          msg.ToUserId.Str,
			Content:           msg.Content.Str,
		}

		if strings.HasSuffix(message.FromUserId, "@chatroom") {
			message.ChatType = ChatTypeGroup
		} else {
			message.ChatType = ChatTypePrivate
		}

		if message.ChatType == ChatTypeGroup {
			groupId := message.FromUserId
			splited := strings.SplitN(message.Content, ":\n", 2)
			if len(splited) == 2 {
				message.FromGroupId = groupId
				message.FromUserId = splited[0]
				message.Content = splited[1]
			} else {
				logger.Warn("Failed to split group message", slog.String("Content", message.Content))
			}
		}
		for _, handler := range w.handlers {
			if handler(message) {
				break
			}
		}
		return true
	})
}

func (w *WechatClient) Start() error {
	w.initAccount()
	w.syncMessage()
	return w.ws.Listen()
}

func (w *WechatClient) Stop() {
	w.ws.Close()
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
