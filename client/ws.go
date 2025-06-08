package client

import (
	"context"
	"focalors-go/slogger"
	"log/slog"
	"net/url"
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
						logger.Info("ContentData", slog.String("Data", content.Data.(string)[:10]))
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
