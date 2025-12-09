package transit

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/frostbyte57/GoPipe/internal/crypto"
)

type Metadata struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Mode string `json:"mode"` // "file" or "dir"
}

// Transit handles the data connection.
type Transit struct {
	sessionKey []byte
	listener   net.Listener
	conn       net.Conn
	localHints []string
}

type TransitMessage struct {
	Hints []string `json:"hints"`
}

func NewTransit(sessionKey []byte) *Transit {
	return &Transit{
		sessionKey: sessionKey,
	}
}

// Start prepares the transit: listens on a port and returns our hints.
func (t *Transit) Start() ([]string, error) {
	// Listen on random port
	l, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return nil, err
	}
	t.listener = l

	// Get local IPs
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	port := l.Addr().(*net.TCPAddr).Port
	var hints []string

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				hints = append(hints, fmt.Sprintf("%s:%d", ipnet.IP.String(), port))
			}
		}
	}
	// Also add localhost for local testing
	hints = append(hints, fmt.Sprintf("127.0.0.1:%d", port))

	t.localHints = hints

	// Start accepting in background
	go t.acceptLoop()

	return hints, nil
}

func (t *Transit) acceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			return
		}
		// In a real implementation, we would handshake here to confirm it's the right peer.
		// For this MVP, we assume the first connection is the peer (RISKY but simple).
		// Better: Exchange a nonce encrypted with sessionKey.
		t.conn = conn
		t.listener.Close() // Stop listening once connected
		return
	}
}

// ConnectToPeer tries to connect to one of the peer hints.
func (t *Transit) ConnectToPeer(ctx context.Context, hints []string) error {
	// Try establishing connection to hints. All in parallel or serial?
	// Serial for simplicity.

	// If we already accepted a connection (race), we use that.
	if t.conn != nil {
		return nil
	}

	// Logic: We should try to connect. If we succeed, we use it.
	// Validating the connection is crucial.

	for _, hint := range hints {
		d := net.Dialer{Timeout: 2 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", hint)
		if err == nil {
			t.conn = conn
			if t.listener != nil {
				t.listener.Close()
			}
			return nil
		}
	}

	// Wait for incoming connection if strictly peer-to-peer faltered?
	// In this simplified logic, we just wait a bit to see if acceptLoop got something.
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("failed to connect to hints and no incoming connection")
		case <-ticker.C:
			if t.conn != nil {
				return nil
			}
		}
	}
}

// Encrypt/Decrypt stream logic would go here.
// For now, we will just treat the connection as the secure stream
// (assuming we wrap it in a secure channel, but we just have raw TCP here).
// REAL IMPLEMENTATION MUST WRAP THIS.
// I will implement a basic EncryptedConn wrapper using the session key.

func (t *Transit) SecureConnection() (io.ReadWriteCloser, error) {
	if t.conn == nil {
		return nil, fmt.Errorf("no connection")
	}
	// TODO: Wrap with NaCl SecretBox stream?
	// A simple block-based framing: [4 bytes len][nonce][ciphertext]
	return &EncryptedConn{
		conn: t.conn,
		key:  t.sessionKey,
	}, nil
}

type EncryptedConn struct {
	conn net.Conn
	key  []byte
	buf  []byte // read buffer
}

func (ec *EncryptedConn) Write(p []byte) (n int, err error) {
	// Encrypt chunk
	encrypted, err := crypto.Encrypt(ec.key, p)
	if err != nil {
		return 0, err
	}
	// Send Length (4 bytes) + Content
	length := uint32(len(encrypted))
	header := make([]byte, 4)
	header[0] = byte(length >> 24)
	header[1] = byte(length >> 16)
	header[2] = byte(length >> 8)
	header[3] = byte(length)

	_, err = ec.conn.Write(header)
	if err != nil {
		return 0, err
	}
	_, err = ec.conn.Write(encrypted)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (ec *EncryptedConn) Read(p []byte) (n int, err error) {
	// If we have buffered data, return it
	if len(ec.buf) > 0 {
		copyLen := copy(p, ec.buf)
		ec.buf = ec.buf[copyLen:]
		return copyLen, nil
	}

	// Read Header
	header := make([]byte, 4)
	if _, err := io.ReadFull(ec.conn, header); err != nil {
		return 0, err
	}
	length := uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])

	// Limit max size
	if length > 100*1024*1024 { // 100MB chunk max
		return 0, fmt.Errorf("chunk too large")
	}

	encrypted := make([]byte, length)
	if _, err := io.ReadFull(ec.conn, encrypted); err != nil {
		return 0, err
	}

	decrypted, err := crypto.Decrypt(ec.key, encrypted)
	if err != nil {
		return 0, err
	}

	copyLen := copy(p, decrypted)
	if copyLen < len(decrypted) {
		ec.buf = decrypted[copyLen:]
	}
	return copyLen, nil
}

func (ec *EncryptedConn) Close() error {
	return ec.conn.Close()
}
