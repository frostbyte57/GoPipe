package ui

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/frostbyte57/GoPipe/internal/transit"
	"github.com/frostbyte57/GoPipe/internal/wormhole"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SendModel struct {
	client      *wormhole.Client
	textInput   textinput.Model
	progressBar progress.Model
	code        string
	status      string
	progress    float64
	err         error
	sending     bool
	uploading   bool // separate state for actual data transfer
	done        bool
	transferSub TransferStartedMsg
	mailboxURL  string
}

func NewSendModel(mailboxURL string) SendModel {
	ti := textinput.New()
	ti.Placeholder = "/path/to/file"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40
	// Style input
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ColorGoBlue)

	prog := progress.New(progress.WithDefaultGradient())
	prog.Width = 40

	return SendModel{
		textInput:   ti,
		progressBar: prog,
		status:      "Enter file path:",
		mailboxURL:  mailboxURL,
	}
}

func (m SendModel) Init() tea.Cmd {
	return textinput.Blink
}

type CodeGeneratedMsg string
type ConnectedMsg struct {
	Code   string
	Client *wormhole.Client
}
type HandshakeSuccessMsg []byte
type ProgressMsg float64
type TransferDoneMsg struct{}
type ErrorMsg error

func (m SendModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if !m.sending && !m.done {
				filePath := m.textInput.Value()
				m.sending = true
				m.status = "Connecting..."
				return m, startSend(filePath, m.mailboxURL)
			}
		case tea.KeyEsc:
			return m, func() tea.Msg { return BackToMenuMsg{} }
		}

	case ConnectedMsg:
		m.code = msg.Code
		m.client = msg.Client
		m.status = fmt.Sprintf("Code: %s\nWaiting for receiver...", m.code)
		return m, waitForReceiver(m.client, m.textInput.Value(), 0)

	case HandshakeSuccessMsg:
		m.status = "Connected! Sending..."
		m.uploading = true
		// Trigger the actual transfer
		return m, startTransfer(m.client, m.textInput.Value())

	case TransferStartedMsg:
		m.transferSub = msg
		return m, listenTransfer(msg)

	case ProgressMsg:
		var cmds []tea.Cmd
		m.progress = float64(msg)

		if m.progress >= 1.0 {
			cmd := m.progressBar.SetPercent(1.0)
			cmds = append(cmds, cmd)
		} else {
			cmd := m.progressBar.SetPercent(m.progress)
			cmds = append(cmds, cmd)
		}
		// Continue listening!
		// We need to keep polling the channel.
		// Since `listenTransfer` returns ONE msg, we need to re-queue it.
		// But `listenTransfer` takes the struct with channels. We didn't store it.
		// We need to store the channels in the model or return a command that has them enclosed.
		// Actually, `listenTransfer` is a closure if we implemented it right?
		// No, `listenTransfer(msg)` uses the message content.
		// So we must store `TransferStartedMsg` in the model to reuse it?
		// OR: The `listenTransfer` command can return a `Msg` that includes the channels for the NEXT step.
		// So we must store `transferSub` in Model.
		cmds = append(cmds, m.waitForNextProgress())
		return m, tea.Batch(cmds...)

	case progress.FrameMsg:
		progressModel, cmd := m.progressBar.Update(msg)
		m.progressBar = progressModel.(progress.Model)
		return m, cmd

	case TransferDoneMsg:
		m.done = true
		m.uploading = false
		m.status = "Transfer Complete!"
		return m, tea.Quit

	case ErrorMsg:
		m.err = msg
		m.status = fmt.Sprintf("Error: %v", msg)
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m SendModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n%s\n\n%s\n\n%s",
			TitleStyle.Render("Error"),
			StatusStyle.Foreground(ColorError).Render(m.err.Error()),
			HelpStyle.Render("Press Esc to retry"),
		)
	}

	if m.done {
		return fmt.Sprintf("\n%s\n\n%s", TitleStyle.Render("Success"), StatusStyle.Foreground(ColorSuccess).Render(m.status))
	}

	if m.uploading {
		// Show progress bar
		return fmt.Sprintf("\n%s\n\n%s\n\n%s",
			TitleStyle.Render("Sending File..."),
			m.progressBar.View(),
			StatusStyle.Render(fmt.Sprintf("%.0f%%", m.progress*100)),
		)
	}

	if m.code != "" {
		codeBox := CodeBoxStyle.Render(m.code)
		return fmt.Sprintf("\n%s\n%s\n\n%s",
			TitleStyle.Render("Ready to Send"),
			codeBox,
			StatusStyle.Render("Share this code with the receiver."),
		)
	}

	// Input State
	return fmt.Sprintf("\n%s\n\n%s\n\n%s",
		TitleStyle.Render("Send File"),
		m.textInput.View(),
		HelpStyle.Render("Enter absolute path to file"),
	)
}

func startSend(filePath string, mailboxURL string) tea.Cmd {
	return func() tea.Msg {
		file, err := os.Open(filePath)
		if err != nil {
			return ErrorMsg(err)
		}
		stat, _ := file.Stat()
		_ = stat.Size() // We store size later or pass it
		file.Close()

		c := wormhole.NewClient("", mailboxURL)
		ctx := context.Background()

		code, err := c.PrepareSend(ctx)
		if err != nil {
			return ErrorMsg(err)
		}

		return ConnectedMsg{Code: code, Client: c}
	}
}

// Separate command for the rest of the flow...
// This is tricky in Bubble Tea without a state machine manager.
// We can chain commands.
// Update: ConnectedMsg -> Trigger Handshake Cmd.

// Helper to keep listening
func (m SendModel) waitForNextProgress() tea.Cmd {
	return listenTransfer(m.transferSub)
}

func waitForReceiver(c *wormhole.Client, filePath string, fileSize int64) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		_, err := c.PerformHandshake(ctx)
		if err != nil {
			return ErrorMsg(err)
		}

		// CORRECT PATTERN:
		// We need to break this down.
		// `waitForReceiver` should ONLY do handshake.
		// Then return `HandshakeSuccess`.

		// The `PerformTransfer` (which negotiates) also needs to happen.
		// Let's assume PerformTransfer is fast (negotiation).
		// The DATA transfer is slow.

		return HandshakeSuccessMsg(nil)
	}
}

func startTransfer(c *wormhole.Client, filePath string) tea.Cmd {
	return func() tea.Msg {
		// We need to do the transfer here.
		// But how to stream progress?
		// We can use a `tea.Program` if we pass it, but we can't.

		// Creating a channel for progress
		progressChan := make(chan float64, 100)
		errChan := make(chan error, 1)
		doneChan := make(chan struct{})

		go func() {
			defer close(progressChan)
			defer close(doneChan)

			// We need to call PerformTransfer here if not called yet?
			// Ideally we separate Handshake and Transfer in Client.
			// Re-calling PerformTransfer might re-negotiate.
			// Let's assume `waitForReceiver` returned after Handshake.
			// We need `PerformTransfer` to get the connection.

			ctx := context.Background()
			conn, err := c.PerformTransfer(ctx)
			if err != nil {
				errChan <- err
				return
			}
			defer conn.Close()

			info, err := os.Stat(filePath)
			if err != nil {
				errChan <- err
				return
			}

			// Determine mode and output stream
			mode := "file"
			if info.IsDir() {
				mode = "dir"
			}

			// If dir, we zip it. If file, send as is.
			// But wait, user wants original format.
			// We always send metadata first.

			var reader io.Reader
			var size int64
			var name string

			if mode == "dir" {
				// Create a pipe to stream zip
				pr, pw := io.Pipe()
				reader = pr
				// We don't know total zip size ahead of time without buffering.
				// For progress bar:
				// 1. Walk dir to count total bytes of files.
				// 2. Use that as estimation for progress.

				// Calculate total size of files to zip
				name = filepath.Base(filePath) + ".zip"
				// size = ??? (unknown compressed size)
				// We'll set size to 0 or estimated uncompressed size?
				// Receiver uses size for progress.
				// Let's set size to total uncompressed size.
				size = 0
				filepath.Walk(filePath, func(_ string, info os.FileInfo, err error) error {
					if !info.IsDir() {
						size += info.Size()
					}
					return nil
				})

				go func() {
					zw := zip.NewWriter(pw)
					baseDir := filepath.Dir(filePath) // parent
					filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}
						// Create header
						header, err := zip.FileInfoHeader(info)
						if err != nil {
							return err
						}
						// modify name to be relative
						relPath, _ := filepath.Rel(baseDir, path)
						header.Name = relPath

						if info.IsDir() {
							header.Name += "/"
						} else {
							header.Method = zip.Deflate
						}

						w, err := zw.CreateHeader(header)
						if err != nil {
							return err
						}
						if !info.IsDir() {
							f, err := os.Open(path)
							if err != nil {
								return err
							}
							defer f.Close()
							io.Copy(w, f)
						}
						return nil
					})
					zw.Close()
					pw.Close()
				}()

			} else {
				// File
				f, err := os.Open(filePath)
				if err != nil {
					errChan <- err
					return
				}
				defer f.Close()
				reader = f
				size = info.Size()
				name = info.Name()
			}

			// Send Metadata
			meta := transit.Metadata{
				Name: name,
				Size: size,
				Mode: mode,
			}
			metaBytes, _ := json.Marshal(meta)

			// Frame it: [4 bytes len][json]
			// We can reuse EncryptedConn but we need to send this raw bytes via conn first?
			// Wait, conn IS EncryptedConn (io.ReadWriteCloser interface).
			// So we just write to it.

			// Write Metadata Length (4 bytes)
			metaLen := uint32(len(metaBytes))
			lenBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(lenBuf, metaLen)

			if _, err := conn.Write(lenBuf); err != nil {
				errChan <- err
				return
			}
			if _, err := conn.Write(metaBytes); err != nil {
				errChan <- err
				return
			}

			// Send Content
			buf := make([]byte, 32*1024)
			var current int64

			for {
				n, err := reader.Read(buf)
				if n > 0 {
					_, wErr := conn.Write(buf[:n])
					if wErr != nil {
						errChan <- wErr
						return
					}
					current += int64(n)
					if size > 0 {
						progressChan <- float64(current) / float64(size)
					}
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					errChan <- err
					return
				}
			}
		}()

		return TransferStartedMsg{
			ProgressChan: progressChan,
			ErrChan:      errChan,
			DoneChan:     doneChan,
		}
	}
}

type TransferStartedMsg struct {
	ProgressChan <-chan float64
	ErrChan      <-chan error
	DoneChan     <-chan struct{}
}

// Command to listen to the channels
func listenTransfer(sub TransferStartedMsg) tea.Cmd {
	return func() tea.Msg {
		select {
		case p, ok := <-sub.ProgressChan:
			if !ok {
				return TransferDoneMsg{}
			}
			return ProgressMsg(p)
		case err := <-sub.ErrChan:
			return ErrorMsg(err)
		case <-sub.DoneChan:
			return TransferDoneMsg{}
		}
	}
}
