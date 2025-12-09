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

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ReceiveModel struct {
	client     *wormhole.Client
	textInput  textinput.Model
	status     string
	receiving  bool
	done       bool
	err        error
	mailboxURL string
}

func NewReceiveModel(mailboxURL string) ReceiveModel {
	ti := textinput.New()
	ti.Placeholder = "7-code-words"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ColorGoBlue)

	return ReceiveModel{
		textInput:  ti,
		status:     "Enter Wormhole Code:",
		mailboxURL: mailboxURL,
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
				m.status = "Connnecting..."
				return m, startReceive(code, m.mailboxURL)
			}
		case tea.KeyEsc:
			return m, func() tea.Msg { return BackToMenuMsg{} }
		}

	case HandshakeSuccessMsg:
		m.status = "Connected! Receiving..."
		return m, nil

	case ErrorMsg:
		m.err = msg
		m.receiving = false
		return m, nil

	case TransferDoneMsg:
		m.done = true
		m.status = "Received File! (saved as 'received_file')"
		return m, tea.Quit
	}

	// Retry logic
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

	// Input State
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

		conn, err := c.PerformTransfer(ctx)
		if err != nil {
			return ErrorMsg(err)
		}
		defer conn.Close()

		// Receive Metadata
		// 1. Read 4 bytes length
		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return ErrorMsg(err)
		}
		metaLen := binary.BigEndian.Uint32(lenBuf)

		// 2. Read Metadata JSON
		metaBuf := make([]byte, metaLen)
		if _, err := io.ReadFull(conn, metaBuf); err != nil {
			return ErrorMsg(err)
		}

		var meta transit.Metadata
		if err := json.Unmarshal(metaBuf, &meta); err != nil {
			return ErrorMsg(err)
		}

		// Determine Output Path
		// Load config or use default
		cfg, _ := config.LoadConfig()
		outDir := "."
		if cfg != nil && cfg.DownloadDir != "" {
			outDir = cfg.DownloadDir
		}

		outPath := filepath.Join(outDir, meta.Name)

		// Receive file content
		out, err := os.Create(outPath)
		if err != nil {
			return ErrorMsg(err)
		}
		defer out.Close()

		_, err = io.Copy(out, conn)
		if err != nil {
			return ErrorMsg(err)
		}

		return TransferDoneMsg{}
	}
}
