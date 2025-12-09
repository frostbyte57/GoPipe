package ui

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/frostbyte57/GoPipe/internal/config"
	"github.com/frostbyte57/GoPipe/internal/transit"
	"github.com/frostbyte57/GoPipe/internal/wormhole"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ReceiveModel struct {
	client        *wormhole.Client
	textInput     textinput.Model
	progressBar   progress.Model
	status        string
	receiving     bool
	transferring  bool
	done          bool
	err           error
	mailboxURL    string
	progress      float64
	receivedBytes int64
	totalBytes    int64
	transferSub   ReceiveTransferStartedMsg
}

type ReceiveTransferStartedMsg struct {
	ProgressChan <-chan TxProgressMsg
	ErrChan      <-chan error
	ResultChan   <-chan string
}

func NewReceiveModel(mailboxURL string) ReceiveModel {
	ti := textinput.New()
	ti.Placeholder = "7-code-words"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ColorGoBlue)

	prog := progress.New(progress.WithDefaultGradient())
	prog.Width = 40
	prog.ShowPercentage = false

	return ReceiveModel{
		textInput:   ti,
		progressBar: prog,
		status:      "Enter Wormhole Code:",
		mailboxURL:  mailboxURL,
	}
}

func (m ReceiveModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ReceiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if !m.receiving && !m.done {
				code := m.textInput.Value()
				m.receiving = true
				m.status = "Connecting..."
				return m, startReceive(code, m.mailboxURL)
			}
		case tea.KeyEsc:
			return m, func() tea.Msg { return BackToMenuMsg{} }
		}

	case ConnectedMsg:
		m.client = msg.Client
		m.status = "Connected! Receiving..."
		m.transferring = true
		return m, startReceiveTransfer(m.client)

	case ReceiveTransferStartedMsg:
		m.transferSub = msg
		return m, listenReceiveTransfer(msg)

	case TxProgressMsg:
		var cmds []tea.Cmd
		m.progress = msg.Ratio
		m.receivedBytes = msg.Current
		m.totalBytes = msg.Total

		if m.progress >= 1.0 {
			cmd := m.progressBar.SetPercent(1.0)
			cmds = append(cmds, cmd)
		} else {
			cmd := m.progressBar.SetPercent(m.progress)
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.waitForNextReceiveProgress())
		return m, tea.Batch(cmds...)

	case progress.FrameMsg:
		progressModel, cmd := m.progressBar.Update(msg)
		m.progressBar = progressModel.(progress.Model)
		return m, cmd

	case TransferDoneMsg:
		m.done = true
		m.receiving = false
		m.transferring = false
		m.status = fmt.Sprintf("Received File! (saved as '%s')", msg.Filename)
		return m, tea.Quit

	case ErrorMsg:
		m.err = msg
		m.receiving = false
		m.transferring = false
		return m, nil
	}

	if m.err != nil {
		if msg, ok := msg.(tea.KeyMsg); ok && msg.Type == tea.KeyEsc {
			m.err = nil
			m.status = "Enter Wormhole Code:"
			m.textInput.SetValue("")
			m.textInput.Focus()
			return m, nil
		}
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m ReceiveModel) View() string {
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

	if m.transferring {
		return fmt.Sprintf("\n%s\n\n%s\n\n%s",
			TitleStyle.Render("Receiving File..."),
			m.progressBar.View(),
			StatusStyle.Render(fmt.Sprintf("%s / %s (%.0f%%)",
				byteCountBinary(m.receivedBytes),
				byteCountBinary(m.totalBytes),
				m.progress*100)),
		)
	}

	return fmt.Sprintf("\n%s\n\n%s\n\n%s",
		TitleStyle.Render("Receive File"),
		m.textInput.View(),
		HelpStyle.Render("Enter Wormhole Code (e.g. 7-words)"),
	)
}

func startReceive(code string, mailboxURL string) tea.Cmd {
	return func() tea.Msg {
		c := wormhole.NewClient("", mailboxURL)
		ctx := context.Background()

		if err := c.PrepareReceive(ctx, code); err != nil {
			return ErrorMsg(err)
		}

		_, err := c.PerformHandshake(ctx)
		if err != nil {
			return ErrorMsg(err)
		}

		return ConnectedMsg{Code: code, Client: c}
	}
}

func (m ReceiveModel) waitForNextReceiveProgress() tea.Cmd {
	return listenReceiveTransfer(m.transferSub)
}

func startReceiveTransfer(c *wormhole.Client) tea.Cmd {
	return func() tea.Msg {
		progressChan := make(chan TxProgressMsg, 100)
		errChan := make(chan error, 1)
		resultChan := make(chan string)

		go func() {
			defer close(progressChan)
			defer close(resultChan)

			ctx := context.Background()
			conn, err := c.PerformTransfer(ctx)
			if err != nil {
				errChan <- err
				return
			}
			defer conn.Close()

			lenBuf := make([]byte, 4)
			if _, err := io.ReadFull(conn, lenBuf); err != nil {
				errChan <- err
				return
			}
			metaLen := binary.BigEndian.Uint32(lenBuf)

			metaBuf := make([]byte, metaLen)
			if _, err := io.ReadFull(conn, metaBuf); err != nil {
				errChan <- err
				return
			}

			var meta transit.Metadata
			if err := json.Unmarshal(metaBuf, &meta); err != nil {
				errChan <- err
				return
			}

			cfg, _ := config.LoadConfig()
			outDir := "."
			if cfg != nil && cfg.DownloadDir != "" {
				outDir = cfg.DownloadDir
			}

			outPath := filepath.Join(outDir, meta.Name)
			out, err := os.Create(outPath)
			if err != nil {
				errChan <- err
				return
			}
			defer out.Close()

			var received int64
			buf := make([]byte, 32*1024)
			for {
				n, rErr := conn.Read(buf)
				if n > 0 {
					_, wErr := out.Write(buf[:n])
					if wErr != nil {
						errChan <- wErr
						return
					}
					received += int64(n)
					if meta.Size > 0 {
						ratio := float64(received) / float64(meta.Size)
						progressChan <- TxProgressMsg{
							Current: received,
							Total:   meta.Size,
							Ratio:   ratio,
						}
					}
				}
				if rErr == io.EOF {
					break
				}
				if rErr != nil {
					errChan <- rErr
					return
				}
			}

			// Send filename to resultChan
			resultChan <- meta.Name
		}()

		return ReceiveTransferStartedMsg{
			ProgressChan: progressChan,
			ErrChan:      errChan,
			ResultChan:   resultChan,
		}
	}
}

func listenReceiveTransfer(sub ReceiveTransferStartedMsg) tea.Cmd {
	return func() tea.Msg {
		select {
		case p, ok := <-sub.ProgressChan:
			if !ok {
				// Progress channel closed. Wait for result.
				name, ok := <-sub.ResultChan
				if !ok {
					return nil // Should not happen if successful
				}
				return TransferDoneMsg{Filename: name}
			}
			return p
		case err := <-sub.ErrChan:
			return ErrorMsg(err)
		case name := <-sub.ResultChan:
			return TransferDoneMsg{Filename: name}
		}
	}
}
