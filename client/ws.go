package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"focalors-go/slogger"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var logger = slogger.New("client")

// A websocket client
type WebSocketClient struct {
	Conn *websocket.Conn
	Url  string
}

// New creates a new WebSocket client talking to Yunzai
func New(url string) *WebSocketClient {
	c := &WebSocketClient{
		Url: url,
	}
	return c
}

// Connect connects to the websocket server
func (c *WebSocketClient) Connect() error {
	logger.Info("Connecting to websocket server", slog.Any("url", c.Url))
	// Parse the provided URL
	u, err := url.Parse(c.Url)
	if err != nil {
		return err
	}
	// Create a default dialer and dial the server
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	c.Conn = conn
	return nil
}

func (c *WebSocketClient) Send(message any) error {
	err := c.Conn.WriteJSON(message)
	if err != nil {
		return err
	}
	return nil
}

func (c *WebSocketClient) Close() {
	if c.Conn != nil {
		c.Conn.Close()
		c.Conn = nil
	}
}

func (c *WebSocketClient) Stop() {
	c.Close()
}

func (c *WebSocketClient) Listen(ctx context.Context) chan Response {
	received := make(chan Response, 10)
	go func() {
		defer close(received)
		for {
			select {
			case <-ctx.Done():
				logger.Info("Context canceled, stopping listening")
				return
			default:
				message := Response{}
				err := c.Conn.ReadJSON(&message)
				if err != nil {
					logger.Error("Failed to read message", slog.Any("error", err))
					if websocket.IsUnexpectedCloseError(err) {
						// websocket connection is closed
						return
					}
					// sleep for a while before trying again
					time.Sleep(2 * time.Second)
					continue
				}
				logger.Info("Received message",
					slog.String("BotId", message.BotSelfId),
					slog.String("MsgId", message.MsgId),
					slog.String("TargetId", message.TargetId),
				)
				for _, content := range message.Content {
					logger.Info("ContentType", slog.String("Type", content.Type))
					if content.Type == "image" {
						// Try to display the image if content.Data is a base64 string (e.g., base64:///9j/4AAQ...)
						if dataUrl, ok := content.Data.(string); ok && strings.HasPrefix(dataUrl, "base64://") {
							base64Data := strings.TrimPrefix(dataUrl, "base64://")
							imgBytes, err := base64.StdEncoding.DecodeString(base64Data)
							if err == nil {
								fileName := "output-" + time.Now().Format("20060102-150405") + ".png"
								err = os.WriteFile(fileName, imgBytes, 0644)
								if err == nil {
									logger.Info("Image saved", slog.String("file", fileName))
									if os.Getenv("TERM_PROGRAM") == "iTerm.app" {
										fmt.Printf("\033]1337;File=inline=1;width=auto;height=auto;preserveAspectRatio=1:%s\a\n", base64Data)
									}
								} else {
									logger.Error("Failed to write image file", slog.Any("error", err))
								}
							} else {
								logger.Error("Failed to decode base64 image", slog.Any("error", err))
							}
						}
						continue
					}
					logger.Info("ContentData", slog.Any("Data", content.Data))
				}
				received <- message
			}
		}
	}()
	return received
}
