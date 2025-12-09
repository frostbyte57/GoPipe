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
	sentBytes   int64
	totalBytes  int64
	err         error
	sending     bool
	uploading   bool
	done        bool
	transferSub TransferStartedMsg
	mailboxURL  string
}

type TransferStartedMsg struct {
	ProgressChan <-chan TxProgressMsg
	ErrChan      <-chan error
	DoneChan     <-chan struct{}
}

func NewSendModel(mailboxURL string) SendModel {
	ti := textinput.New()
	ti.Placeholder = "/path/to/file"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ColorGoBlue)

	prog := progress.New(progress.WithDefaultGradient())
	prog.Width = 40
	prog.ShowPercentage = false

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
		return m, startTransfer(m.client, m.textInput.Value())

	case TransferStartedMsg:
		m.transferSub = msg
		return m, listenTransfer(msg)

	case TxProgressMsg:
		var cmds []tea.Cmd
		m.progress = msg.Ratio
		m.sentBytes = msg.Current
		m.totalBytes = msg.Total

		if m.progress >= 1.0 {
			cmd := m.progressBar.SetPercent(1.0)
			cmds = append(cmds, cmd)
		} else {
			cmd := m.progressBar.SetPercent(m.progress)
			cmds = append(cmds, cmd)
		}
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
		return fmt.Sprintf("\n%s\n\n%s\n\n%s",
			TitleStyle.Render("Sending File..."),
			m.progressBar.View(),
			StatusStyle.Render(fmt.Sprintf("%s / %s (%.0f%%)",
				byteCountBinary(m.sentBytes),
				byteCountBinary(m.totalBytes),
				m.progress*100)),
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
		_ = stat.Size()
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
		return HandshakeSuccessMsg(nil)
	}
}

func startTransfer(c *wormhole.Client, filePath string) tea.Cmd {
	return func() tea.Msg {
		progressChan := make(chan TxProgressMsg, 100)
		errChan := make(chan error, 1)
		doneChan := make(chan struct{})

		go func() {
			defer close(progressChan)
			defer close(doneChan)

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

			mode := "file"
			if info.IsDir() {
				mode = "dir"
			}

			var reader io.Reader
			var size int64
			var name string

			if mode == "dir" {
				pr, pw := io.Pipe()
				reader = pr
				name = filepath.Base(filePath) + ".zip"
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
						header, err := zip.FileInfoHeader(info)
						if err != nil {
							return err
						}
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

			meta := transit.Metadata{
				Name: name,
				Size: size,
				Mode: mode,
			}
			metaBytes, _ := json.Marshal(meta)

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
						ratio := float64(current) / float64(size)
						progressChan <- TxProgressMsg{
							Current: current,
							Total:   size,
							Ratio:   ratio,
						}
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

func listenTransfer(sub TransferStartedMsg) tea.Cmd {
	return func() tea.Msg {
		select {
		case p, ok := <-sub.ProgressChan:
			if !ok {
				return TransferDoneMsg{}
			}
			return p
		case err := <-sub.ErrChan:
			return ErrorMsg(err)
		case <-sub.DoneChan:
			return TransferDoneMsg{}
		}
	}
}

func byteCountBinary(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
