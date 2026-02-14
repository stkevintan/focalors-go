package protocol

import (
	"context"
	"errors"
	"focalors-go/slogger"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var wsLogger = slogger.New("protocol.websocket")

// A websocket client
type WebSocketClient[Message any] struct {
	Conn          *websocket.Conn
	Url           string
	messageBuffer chan Message
	wg            sync.WaitGroup
	onConnect     func()
}

// New creates a new WebSocket client
func NewClient[Message any](url string) *WebSocketClient[Message] {
	return &WebSocketClient[Message]{
		Url:           url,
		messageBuffer: make(chan Message, 5), // Reduced from 20 to 5
	}
}

// OnConnect registers a callback that fires after each successful connection (including reconnects).
func (c *WebSocketClient[Message]) OnConnect(fn func()) {
	c.onConnect = fn
}

// Connect connects to the websocket server
func (c *WebSocketClient[Message]) Connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(c.Url, nil)
	if err != nil {
		c.Conn = nil // Ensure Conn is nil on failure
		wsLogger.Error("[WebSocket] Failed to dial", slog.String("url", c.Url), slog.Any("error", err))
		return err
	}

	c.Conn = conn
	wsLogger.Info("[WebSocket] Successfully connected.", slog.String("url", c.Url))
	if c.onConnect != nil {
		go c.onConnect()
	}
	return nil
}

func (c *WebSocketClient[Message]) Send(message any) error {
	if c.Conn == nil {
		wsLogger.Error("[WebSocket] Connection is nil, cannot send message.", slog.String("url", c.Url))
		return errors.New("connection is nil")
	}
	err := c.Conn.WriteJSON(message)
	if err != nil {
		wsLogger.Error("[WebSocket] Failed to write JSON", slog.String("url", c.Url), slog.Any("error", err))
		return err
	}
	return nil
}

func (c *WebSocketClient[Message]) Listen(ctx context.Context) error {
	defer close(c.messageBuffer)
	for {
		select {
		case <-ctx.Done():
			wsLogger.Info("[WebSocket] Context done, exiting message loop.")
			c.close() // Ensure connection is closed and status updated on context done
			return ctx.Err()
		default:
			var message Message
			if c.Conn == nil {
				wsLogger.Warn("[WebSocket] Connection is nil, attempting to reconnect.", slog.String("url", c.Url))
				if err := c.Connect(); err != nil {
					wsLogger.Error("[WebSocket] Failed to reconnect", slog.String("url", c.Url), slog.Any("error", err))
					time.Sleep(2 * time.Second) // Sleep before reconnecting
					continue
				}
			}

			err := c.Conn.ReadJSON(&message)

			if err == nil {
				// Step 3: Process the successfully read message.
				select {
				case c.messageBuffer <- message:
					// Message sent successfully
				case <-ctx.Done():
					wsLogger.Info("[WebSocket] Context done while attempting to send message to channel.", slog.String("url", c.Url))
					return ctx.Err()
				case <-time.After(1 * time.Second): // Timeout to prevent blocking Listen indefinitely
					wsLogger.Warn("[WebSocket] Timeout sending message to processing channel. Channel might be full or processor stuck.", slog.String("url", c.Url))
				}
				continue
			}

			if isTerminalError(err) {
				wsLogger.Warn("[WebSocket] Terminal error occurred", slog.String("url", c.Url), slog.Any("error", err))
				c.Conn = nil // Reset connection
				continue
			}
			time.Sleep(2 * time.Second) // Sleep before reconnecting
		}
	}
}

func (c *WebSocketClient[Message]) close() {
	if c.Conn != nil {
		wsLogger.Info("[WebSocket] Closing connection.", slog.String("url", c.Url))
		c.Conn.Close() // Attempt to close
		c.Conn = nil
	}
	c.wg.Wait() // Wait for message processing to finish
	wsLogger.Info("[WebSocket] Connection closed.", slog.String("url", c.Url))
}

func (c *WebSocketClient[Message]) Run(ctx context.Context, OnMessage func(msg *Message)) error {
	c.wg.Add(1)
	go c.processMessages(ctx, OnMessage)
	return c.Listen(ctx)
}

func (c *WebSocketClient[Message]) processMessages(ctx context.Context, OnMessage func(msg *Message)) {
	defer c.wg.Done()
	for {
		select {
		case <-ctx.Done():
			wsLogger.Info("[WebSocket] Context done, exiting message processing loop.")
			return
		case message, ok := <-c.messageBuffer:
			if !ok {
				wsLogger.Warn("[WebSocket] Message buffer closed, exiting message processing loop.")
				return
			}
			OnMessage(&message)
		}
	}
}

func isTerminalError(err error) bool {
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
		wsLogger.Info("[WebSocket] Connection closed by peer (CloseError)", slog.Any("error", err))
		return true
	} else if _, ok := err.(*net.OpError); ok || // Covers many net errors
		errors.Is(err, net.ErrClosed) || // Explicitly check for net.ErrClosed
		strings.Contains(err.Error(), "use of closed network connection") ||
		err.Error() == "EOF" { // EOF can also indicate a closed connection
		wsLogger.Error("[WebSocket] Network error or closed connection during ReadJSON", slog.Any("error", err))
		return true
	}
	return false
}
