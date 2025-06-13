package client

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

var logger = slogger.New("client")

// A websocket client
type WebSocketClient[Message any] struct {
	ctx           context.Context
	Conn          *websocket.Conn
	Url           string
	readJSON      func(Conn *websocket.Conn, v interface{}) error
	handlers      []func(msg *Message) bool
	messageBuffer chan Message
	wg            sync.WaitGroup
}

// New creates a new WebSocket client
func NewClient[Message any](ctx context.Context, url string) *WebSocketClient[Message] {
	return &WebSocketClient[Message]{
		ctx:           ctx,
		Url:           url,
		messageBuffer: make(chan Message, 20),
	}
}

func (c *WebSocketClient[Message]) SetReadJSON(readJSON func(Conn *websocket.Conn, v any) error) {
	c.readJSON = readJSON
}

// thread unsafe, but we only call this before starting the client
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
	defer close(c.messageBuffer)
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
				select {
				case c.messageBuffer <- message:
					// Message sent successfully
				case <-c.ctx.Done():
					logger.Info("[WebSocket] Context done while attempting to send message to channel.", slog.String("url", c.Url))
					return c.ctx.Err()
				case <-time.After(1 * time.Second): // Timeout to prevent blocking Listen indefinitely
					logger.Warn("[WebSocket] Timeout sending message to processing channel. Channel might be full or processor stuck.", slog.String("url", c.Url))
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
	c.wg.Wait() // Wait for message processing to finish
	logger.Info("[WebSocket] Connection closed.", slog.String("url", c.Url))
}

func (c *WebSocketClient[Message]) Start() error {
	c.wg.Add(1)
	go c.processMessages()
	return c.Listen()
}

func (c *WebSocketClient[Message]) processMessages() {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			logger.Info("[WebSocket] Context done, exiting message processing loop.")
			return
		case message, ok := <-c.messageBuffer:
			if !ok {
				logger.Warn("[WebSocket] Message buffer closed, exiting message processing loop.")
				return
			}
			for _, handler := range c.handlers {
				if handler(&message) {
					break
				}
			}
		}
	}
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
