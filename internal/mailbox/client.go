package mailbox

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const DefaultURL = "wss://relay.magic-wormhole.io:4000/v1"

type Client struct {
	conn  *websocket.Conn
	url   string
	appID string
	side  string

	EventChan chan interface{}
	errChan   chan error

	closeOnce sync.Once
}

func NewClient(url, appID, side string) *Client {
	if url == "" {
		url = DefaultURL
	}
	return &Client{
		url:       url,
		appID:     appID,
		side:      side,
		EventChan: make(chan interface{}, 100),
		errChan:   make(chan error, 1),
	}
}

func (c *Client) Connect(ctx context.Context) error {
	conn, _, err := websocket.Dial(ctx, c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	c.conn = conn

	// Need to read the "welcome" message first
	var welcome WelcomeMessage
	if err := wsjson.Read(ctx, c.conn, &welcome); err != nil {
		return fmt.Errorf("failed to read welcome: %w", err)
	}
	c.EventChan <- welcome
	// Verify welcome type? Protocol check?

	// Send "bind"
	bind := BindMessage{
		Type:  "bind",
		AppID: c.appID,
		Side:  c.side,
	}
	if err := wsjson.Write(ctx, c.conn, bind); err != nil {
		return fmt.Errorf("failed to send bind: %w", err)
	}

	// Start reading loop
	go c.readLoop()
	return nil
}

func (c *Client) readLoop() {
	// ctx := context.Background() // TODO: inherit or manage
	for {
		var raw json.RawMessage
		// Use a short timeout or context?
		err := wsjson.Read(context.Background(), c.conn, &raw)
		if err != nil {
			// c.errChan <- err
			close(c.EventChan)
			return
		}

		var generic GenericInMessage
		if err := json.Unmarshal(raw, &generic); err != nil {
			continue
		}

		switch generic.Type {
		case "allocated":
			var msg AllocatedMessage
			json.Unmarshal(raw, &msg)
			c.EventChan <- msg
		case "claimed":
			var msg ClaimedMessage
			json.Unmarshal(raw, &msg)
			c.EventChan <- msg
		case "message":
			var msg MessageMessage
			json.Unmarshal(raw, &msg)
			c.EventChan <- msg
		case "welcome":
			// Already handled but if it happens again?
		case "error":
			// TODO: Define ErrorMessage
		case "ack":
			// Ignored
		}
	}
}

// Low-level write
func (c *Client) Write(ctx context.Context, v interface{}) error {
	return wsjson.Write(ctx, c.conn, v)
}

func (c *Client) Allocate(ctx context.Context) error {
	return c.Write(ctx, AllocateMessage{Type: "allocate"})
}

func (c *Client) Claim(ctx context.Context, nameplate string) error {
	return c.Write(ctx, ClaimMessage{Type: "claim", Nameplate: nameplate})
}

func (c *Client) Open(ctx context.Context, mailbox string) error {
	return c.Write(ctx, OpenMessage{Type: "open", Mailbox: mailbox})
}

func (c *Client) Add(ctx context.Context, phase, body string) error {
	return c.Write(ctx, AddMessage{
		Type:  "add",
		Phase: phase,
		Body:  body,
	})
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		if c.conn != nil {
			c.conn.Close(websocket.StatusNormalClosure, "bye")
		}
	})
}
