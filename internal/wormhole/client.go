package wormhole

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/frostbyte57/GoPipe/internal/crypto"
	"github.com/frostbyte57/GoPipe/internal/mailbox"
	"github.com/frostbyte57/GoPipe/internal/transit"
	"github.com/frostbyte57/GoPipe/internal/words"

	"salsa.debian.org/vasudev/gospake2"
)

const (
	AppID = "lothar.com/wormhole/text-or-file-xfer"
)

type Client struct {
	mail  *mailbox.Client
	side  string
	appID string

	code      string
	mailboxID string

	spake2   *gospake2.SPAKE2
	key      []byte // Session key
	isSender bool
}

func NewClient(side string, mailboxURL string) *Client {
	if side == "" {
		b, _ := crypto.RandomBytes(8)
		side = hex.EncodeToString(b)
	}
	return &Client{
		mail:  mailbox.NewClient(mailboxURL, AppID, side),
		side:  side,
		appID: AppID,
	}
}

// PrepareSend connects, allocates a nameplate, and generates a code.
func (c *Client) PrepareSend(ctx context.Context) (code string, err error) {
	c.isSender = true
	if err := c.connect(ctx); err != nil {
		return "", err
	}

	if err := c.mail.Allocate(ctx); err != nil {
		return "", err
	}

	// Wait for "allocated"
	var allocated mailbox.AllocatedMessage
	for {
		select {
		case ev := <-c.mail.EventChan:
			if msg, ok := ev.(mailbox.AllocatedMessage); ok {
				allocated = msg
				goto Allocated
			} else if _, ok := ev.(mailbox.WelcomeMessage); ok {
				continue
			} else {
				return "", fmt.Errorf("unexpected event waiting for allocated: %T", ev)
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
Allocated:

	nameplate := allocated.Nameplate
	id, _ := strconv.Atoi(nameplate)
	c.code = words.GenerateCode(id) // e.g. "7-foo-bar"
	c.mailboxID = nameplate

	if err := c.mail.Open(ctx, nameplate); err != nil {
		return "", err
	}

	return c.code, nil
}

// PrepareReceive connects using a code.
func (c *Client) PrepareReceive(ctx context.Context, code string) error {
	c.isSender = false
	c.code = code

	var nameplate string
	for i, r := range code {
		if r == '-' {
			nameplate = code[:i]
			break
		}
	}
	if nameplate == "" {
		return fmt.Errorf("invalid code format")
	}
	c.mailboxID = nameplate

	if err := c.connect(ctx); err != nil {
		return err
	}

	if err := c.mail.Claim(ctx, nameplate); err != nil {
		return err
	}

	for {
		select {
		case ev := <-c.mail.EventChan:
			if _, ok := ev.(mailbox.ClaimedMessage); ok {
				goto Claimed
			} else if _, ok := ev.(mailbox.WelcomeMessage); ok {
				continue
			} else {
				return fmt.Errorf("unexpected event waiting for claimed: %T", ev)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
Claimed:

	return c.mail.Open(ctx, nameplate)
}

func (c *Client) connect(ctx context.Context) error {
	return c.mail.Connect(ctx)
}

// PerformHashshake executes SPAKE2.
func (c *Client) PerformHandshake(ctx context.Context) (key []byte, err error) {
	pw := gospake2.NewPassword(c.code)
	idA := gospake2.NewIdentityA("sender")
	idB := gospake2.NewIdentityB("receiver")

	var sp gospake2.SPAKE2
	if c.isSender {
		sp = gospake2.SPAKE2A(pw, idA, idB)
	} else {
		sp = gospake2.SPAKE2B(pw, idA, idB)
	}
	c.spake2 = &sp

	msgOut := sp.Start()

	bodyHex := hex.EncodeToString(msgOut)
	if err := c.mail.Add(ctx, "pake", bodyHex); err != nil {
		return nil, err
	}

	// Wait for peer "pake"
	var msgIn mailbox.MessageMessage
	found := false
	for !found {
		select {
		case ev, ok := <-c.mail.EventChan:
			if !ok {
				return nil, fmt.Errorf("channel closed")
			}
			if m, ok := ev.(mailbox.MessageMessage); ok {
				if m.Side == c.side {
					continue
				}
				if m.Phase == "pake" {
					msgIn = m
					found = true
				}
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	peerBody, err := hex.DecodeString(msgIn.Body)
	if err != nil {
		return nil, err
	}

	key, err = sp.Finish(peerBody)
	if err != nil {
		return nil, fmt.Errorf("spake2 handshake failed: %w", err)
	}

	c.key = key
	return key, nil
}

func (c *Client) PerformTransfer(ctx context.Context) (io.ReadWriteCloser, error) {
	t := transit.NewTransit(c.key)
	localHints, err := t.Start()
	if err != nil {
		return nil, fmt.Errorf("transit start failed: %w", err)
	}

	msgStruct := transit.TransitMessage{Hints: localHints}
	msgBytes, _ := json.Marshal(msgStruct)

	encryptedHints, err := crypto.Encrypt(c.key, msgBytes)
	if err != nil {
		return nil, err
	}

	encryptedHex := hex.EncodeToString(encryptedHints)
	if err := c.mail.Add(ctx, "transit", encryptedHex); err != nil {
		return nil, err
	}

	// Wait for phase "transit"
	var peerMsgBytes []byte
	found := false
	for !found {
		select {
		case ev := <-c.mail.EventChan:
			if m, ok := ev.(mailbox.MessageMessage); ok {
				if m.Side == c.side {
					continue
				}
				if m.Phase == "transit" {
					peerMsgBytes, _ = hex.DecodeString(m.Body)
					found = true
				}
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	decryptedHints, err := crypto.Decrypt(c.key, peerMsgBytes)
	if err != nil {
		return nil, fmt.Errorf("decrypt hints failed: %w", err)
	}

	var peerTransitMsg transit.TransitMessage
	if err := json.Unmarshal(decryptedHints, &peerTransitMsg); err != nil {
		return nil, err
	}

	if err := t.ConnectToPeer(ctx, peerTransitMsg.Hints); err != nil {
		return nil, fmt.Errorf("transit connect failed: %w", err)
	}

	return t.SecureConnection()
}
