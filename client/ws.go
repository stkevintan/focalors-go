package client

import (
	"context"
	"errors"
	"focalors-go/slogger"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var logger = slogger.New("client")

// A websocket client
type WebSocketClient[Message any] struct {
	ctx      context.Context
	Conn     *websocket.Conn
	Url      string
	readJSON func(Conn *websocket.Conn, v interface{}) error
	handlers []func(msg *Message) bool
}

// New creates a new WebSocket client
func NewClient[Message any](ctx context.Context, url string) *WebSocketClient[Message] {
	return &WebSocketClient[Message]{
		ctx: ctx,
		Url: url,
	}
}

func (c *WebSocketClient[Message]) SetReadJSON(readJSON func(Conn *websocket.Conn, v any) error) {
	c.readJSON = readJSON
}

func (c *WebSocketClient[Message]) AddMessageHandler(handler func(msg *Message) bool) {
	c.handlers = append(c.handlers, handler)
}

// Connect connects to the websocket server
func (c *WebSocketClient[Message]) Connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(c.Url, nil)
	if err != nil {
		c.Conn = nil // Ensure Conn is nil on failure
		logger.Error("[WebSocket] Failed to dial", slog.String("url", c.Url), slog.Any("error", err))
		return err
	}

	c.Conn = conn
	logger.Info("[WebSocket] Successfully connected.", slog.String("url", c.Url))
	return nil
}

func (c *WebSocketClient[Message]) Send(message any) error {
	if c.Conn == nil {
		logger.Error("[WebSocket] Connection is nil, cannot send message.", slog.String("url", c.Url))
		return errors.New("connection is nil")
	}
	err := c.Conn.WriteJSON(message)
	if err != nil {
		logger.Error("[WebSocket] Failed to write JSON", slog.String("url", c.Url), slog.Any("error", err))
		return err
	}
	return nil
}

func (c *WebSocketClient[Message]) Listen() error {
	for {
		select {
		case <-c.ctx.Done():
			logger.Info("[WebSocket] Context done, exiting message loop.")
			c.Close() // Ensure connection is closed and status updated on context done
			return c.ctx.Err()
		default:
			var message Message
			if c.Conn == nil {
				logger.Warn("[WebSocket] Connection is nil, attempting to reconnect.", slog.String("url", c.Url))
				if err := c.Connect(); err != nil {
					logger.Error("[WebSocket] Failed to reconnect", slog.String("url", c.Url), slog.Any("error", err))
					time.Sleep(2 * time.Second) // Sleep before reconnecting
					continue
				}
			}

			var err error
			if c.readJSON == nil {
				err = c.Conn.ReadJSON(&message)
			} else {
				err = c.readJSON(c.Conn, &message)
			}

			if err == nil {
				// Step 3: Process the successfully read message.
				for _, handler := range c.handlers {
					if handler(&message) {
						break
					}
				}
				continue
			}

			if isTerminalError(err) {
				logger.Warn("[WebSocket] Terminal error occurred", slog.String("url", c.Url), slog.Any("error", err))
				c.Conn = nil // Reset connection
				continue
			}
			time.Sleep(2 * time.Second) // Sleep before reconnecting
		}
	}
}

func (c *WebSocketClient[Message]) Close() {
	if c.Conn != nil {
		logger.Info("[WebSocket] Closing connection.", slog.String("url", c.Url))
		c.Conn.Close() // Attempt to close
		c.Conn = nil
	}
}

func (c *WebSocketClient[Message]) Start() error {
	if err := c.Connect(); err != nil {
		return err
	}
	return c.Listen()
}

func isTerminalError(err error) bool {
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
		logger.Info("[WebSocket] Connection closed by peer (CloseError)", slog.Any("error", err))
		return true
	} else if _, ok := err.(*net.OpError); ok || // Covers many net errors
		errors.Is(err, net.ErrClosed) || // Explicitly check for net.ErrClosed
		strings.Contains(err.Error(), "use of closed network connection") ||
		err.Error() == "EOF" { // EOF can also indicate a closed connection
		logger.Error("[WebSocket] Network error or closed connection during ReadJSON", slog.Any("error", err))
		return true
	}
	return false
}
