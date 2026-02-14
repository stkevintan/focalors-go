package wechat

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

// validateSignature validates the webhook signature using HMAC-SHA256
func validateSignature(body []byte, signature, timestamp, secretKey string) bool {
	if signature == "" {
		return false
	}

	// Calculate expected signature
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(timestamp))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures using constant time comparison
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// StartWebhookServer starts an HTTP server on port 5678 that logs all requests
func (w *WechatClient) StartWebhookServer() error {
	mux := http.NewServeMux()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Handle all routes
	mux.HandleFunc("/webhook/wechat", func(rw http.ResponseWriter, r *http.Request) {
		// Read request body for signature validation
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("Failed to read request body", slog.String("error", err.Error()))
			http.Error(rw, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// Validate signature if secret key is provided
		if w.cfg.WebhookSecret != "" {
			signature := r.Header.Get("X-Webhook-Signature")
			timestamp := r.Header.Get("X-Webhook-Timestamp")
			if !validateSignature(body, signature, timestamp, w.cfg.WebhookSecret) {
				logger.Warn("Invalid signature",
					slog.String("signature", signature),
					slog.String("remote_addr", r.RemoteAddr),
				)
				SetResponse(rw, http.StatusUnauthorized, "error", "Invalid signature")
				return
			}
			slog.Info("Signature validated successfully")
		}
		message := WechatWebHookMessage{}
		err = json.Unmarshal(body, &message)
		if err != nil {
			logger.Error("Failed to unmarshal request body", slog.String("error", err.Error()))
			SetResponse(rw, http.StatusBadRequest, "error", "Failed to unmarshal request body")
			return
		}

		// Log request URL
		// logger.Info("Received request",
		// 	slog.String("method", r.Method),
		// 	slog.String("url", r.URL.String()),
		// 	slog.String("path", r.URL.Path),
		// 	slog.String("query", r.URL.RawQuery),
		// 	slog.String("remote_addr", r.RemoteAddr),
		// 	slog.Any("body", message),
		// )

		SetResponse(rw, http.StatusOK, "ok", "")

		go func(msg WechatWebHookMessage) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Panic in webhook handler", slog.Any("panic", r))
				}
			}()

			select {
			case <-ctx.Done():
				return
			default:
				err := w.handleWebhookMessage(ctx, &msg)
				if err != nil {
					slog.Error("Failed to handle webhook message", slog.String("error", err.Error()))
				}
			}
		}(message)

	})

	slog.Info("Starting webhook server on port 5678")

	server := &http.Server{
		Addr:    ":5678",
		Handler: mux,
	}

	return server.ListenAndServe()
}

func SetResponse(w http.ResponseWriter, status int, result, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"status":  result,
		"message": message,
	})
}

func (w *WechatClient) handleWebhookMessage(ctx context.Context, msg *WechatWebHookMessage) error {
	for _, handler := range w.handlers {
		handler(ctx, msg.Parse())
	}
	return nil
}

func (w *WechatClient) SetWebhook() {
	url := "http://" + w.cfg.WebhookHost + ":5678/webhook/wechat"
	logger.Info("Setting webhook", slog.String("url", url))
	_, err := w.doPostAPICall("/webhook/Config", map[string]any{
		"Enabled":            true,
		"IncludeSelfMessage": true,
		"MessageTypes":       []string{"*"},
		"RetryCount":         0,
		"Secret":             w.cfg.WebhookSecret,
		"Timeout":            0,
		"URL":                url,
	}, nil)
	if err != nil {
		panic("Failed to set webhook: " + err.Error())
	}
	slog.Info("Webhook set successfully", slog.String("url", url))
}
