package wechat

import (
	"context"
	"fmt"
	cfg "focalors-go/config"
	"focalors-go/slogger"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	R "resty.dev/v3"
)

var logger = slogger.New("wechat")

type WechatClient struct {
	cfg        *cfg.WechatConfig
	ctx        context.Context
	httpClient *R.Client
	redis      *redis.Client
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

func NewWechatClient(cfg *cfg.Config, redis *redis.Client, ctx context.Context) *WechatClient {
	httpClient := R.New()
	httpClient.
		SetBaseURL(cfg.Wechat.Server).
		SetDebug(cfg.App.Debug).
		SetTimeout(2*time.Minute).
		SetContext(ctx).
		SetQueryParam("key", cfg.Wechat.Token).
		SetDebugLogFormatter(func(dl *R.DebugLog) string {
			req := fmt.Sprintf("\n-------------\nRequest:\nURL: %s\nHeader: %v\nBody: %s\n", dl.Request.URI, dl.Request.Header, prettyBody(dl.Request.Body))
			res := fmt.Sprintf("---------------\nResponse:\nStatus: %s\nHeader: %v\nBody: %s\n", dl.Response.Status, dl.Response.Header, prettyBody(dl.Response.Body))
			return fmt.Sprintf("%s\n%s", req, res)
		})

	return &WechatClient{
		cfg:        &cfg.Wechat,
		ctx:        ctx,
		httpClient: httpClient,
		redis:      redis,
	}
}

func (w *WechatClient) InitAccount() error {
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

func (w *WechatClient) Start(context context.Context, onMessage func(WechatMessage)) {
	w.InitAccount()
	messageChannel := make(chan WechatMessage, 10)
	defer close(messageChannel)
	go func() {
		for {
			err := w.SubscribeMessage(context, messageChannel)
			if err != nil {
				logger.Error("Failed to subscribe to WeChat messages", slog.Any("error", err))
				time.Sleep(5 * time.Second)
			} else {
				break
			}
		}
	}()

	for msg := range messageChannel {
		onMessage(msg)
	}
}

// 		return err
// 	}

// 	if token != "" {
// 		w.token = token
// 		return nil
// 	}

// 	token, err = w.getNewToken(tokenDays)
// 	if err != nil {
// 		return err
// 	}

// 	w.redis.Set(w.ctx, tokenKey, token, tokenDays*24*time.Hour)
// 	w.token = token
// 	return nil
// }

// func (w *WechatClient) getCachedToken() (string, error) {
// 	token, err := w.redis.Get(w.ctx, "wechat:token").Result()
// 	if err != nil {
// 		return "", err
// 	}
// 	return token, nil
// }

// func (w *WechatClient) getNewToken(days int) (string, error) {
// 	R := w.httpClient.R()
// 	resp, err := R.SetQueryParam("key", w.cfg.AdminKey).SetBody(map[string]any{
// 		"Count": "1",
// 		"Days":  days,
// 	}).SetResult(&ApiResult{}).Post("/admin/GenAuthKey1")

// 	if err != nil {
// 		return "", err
// 	}

// 	if resp.StatusCode() != 200 {
// 		return "", fmt.Errorf("unexpected status code: %s", resp.Status())
// 	}

// 	result := resp.Result().(*ApiResult)
// 	if result.Code != 200 {
// 		return "", fmt.Errorf("API error: %s (%d)", result.Text, result.Code)
// 	}
// 	token := result.Data.([]string)[0]
// 	return token, nil
// }

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
