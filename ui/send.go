package ui

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/frostbyte57/GoPipe/internal/wormhole"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type SendModel struct {
	client    *wormhole.Client
	textInput textinput.Model
	code      string
	status    string
	progress  float64
	err       error
	sending   bool
	done      bool
}

func NewSendModel() SendModel {
	ti := textinput.New()
	ti.Placeholder = "/path/to/file"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40

	return SendModel{
		textInput: ti,
		status:    "Enter file path:",
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
				return m, startSend(filePath)
			}
		case tea.KeyEsc:
			return m, tea.Quit // Or return to main menu
		}

	case ConnectedMsg:
		m.code = msg.Code
		m.client = msg.Client
		m.status = fmt.Sprintf("Code: %s\nWaiting for receiver...", m.code)
		return m, waitForReceiver(m.client, m.textInput.Value(), 0)

	case HandshakeSuccessMsg:
		m.status = "Connected! Sending..."
		return m, nil

	case ProgressMsg:
		m.progress = float64(msg)
		// Update progress bar here if we had one
		return m, nil

	case TransferDoneMsg:
		m.done = true
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
		return fmt.Sprintf("Error: %v\nPress q to quit", m.err)
	}
	if m.done {
		return fmt.Sprintf("%s\n\nPress q to quit", m.status)
	}
	if m.code != "" {
		return fmt.Sprintf("\nPLEASE TELL THE RECEIVER THIS CODE:\n\n    %s\n\n%s", m.code, m.status)
	}
	return fmt.Sprintf("%s\n\n%s", m.status, m.textInput.View())
}

func startSend(filePath string) tea.Cmd {
	return func() tea.Msg {
		file, err := os.Open(filePath)
		if err != nil {
			return ErrorMsg(err)
		}
		stat, _ := file.Stat()
		_ = stat.Size() // We store size later or pass it
		file.Close()

		c := wormhole.NewClient("")
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

func waitForReceiver(c *wormhole.Client, filePath string, fileSize int64) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		_, err := c.PerformHandshake(ctx)
		if err != nil {
			return ErrorMsg(err)
		}

		conn, err := c.PerformTransfer(ctx)
		if err != nil {
			return ErrorMsg(err)
		}
		defer conn.Close()

		// Send file offer?
		// Protocol: We just stream data for this MVP.
		// Send file size first (8 bytes)
		// Or assume stream is file.

		file, err := os.Open(filePath)
		if err != nil {
			return ErrorMsg(err)
		}
		defer file.Close()

		_, err = io.Copy(conn, file)
		if err != nil {
			return ErrorMsg(err)
		}

		return TransferDoneMsg{}
	}
}
