package transit

import (
	"bufio"
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

func (t *Transit) Start() ([]string, error) {
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
	go t.acceptLoop()

	return hints, nil
}

func (t *Transit) acceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			return
		}

		if tcpConn, ok := conn.(*net.TCPConn); ok {
			_ = tcpConn.SetReadBuffer(4 * 1024 * 1024)
			_ = tcpConn.SetWriteBuffer(4 * 1024 * 1024)
			_ = tcpConn.SetNoDelay(true)
		}

		t.conn = conn
		t.listener.Close()
	}
}

func (t *Transit) ConnectToPeer(ctx context.Context, hints []string) error {

	if t.conn != nil {
		return nil
	}

	for _, hint := range hints {
		d := net.Dialer{Timeout: 2 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", hint)
		if err == nil {
			// Tune TCP connection
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				_ = tcpConn.SetReadBuffer(4 * 1024 * 1024)
				_ = tcpConn.SetWriteBuffer(4 * 1024 * 1024)
				_ = tcpConn.SetNoDelay(true)
			}
			t.conn = conn
			if t.listener != nil {
				t.listener.Close()
			}
			return nil
		}
	}

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

func (t *Transit) SecureConnection() (io.ReadWriteCloser, error) {
	if t.conn == nil {
		return nil, fmt.Errorf("no connection")
	}
	return &EncryptedConn{
		conn:      t.conn,
		key:       t.sessionKey,
		msgReader: bufio.NewReaderSize(t.conn, 64*1024*1024),
		msgWriter: bufio.NewWriterSize(t.conn, 64*1024*1024),
	}, nil
}

type EncryptedConn struct {
	conn net.Conn
	key  []byte
	buf  []byte // read buffer for decrypted data (leftover)

	msgReader *bufio.Reader
	msgWriter *bufio.Writer
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

	// Write to buffer
	if _, err := ec.msgWriter.Write(header); err != nil {
		return 0, err
	}
	if _, err := ec.msgWriter.Write(encrypted); err != nil {
		return 0, err
	}
	if err := ec.msgWriter.Flush(); err != nil {
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
	if _, err := io.ReadFull(ec.msgReader, header); err != nil {
		return 0, err
	}
	length := uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])

	if length > 100*1024*1024 { // 100MB chunk max
		return 0, fmt.Errorf("chunk too large")
	}

	encrypted := make([]byte, length)
	if _, err := io.ReadFull(ec.msgReader, encrypted); err != nil {
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
