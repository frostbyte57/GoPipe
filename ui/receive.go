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

type ReceiveModel struct {
	client    *wormhole.Client
	textInput textinput.Model
	status    string
	receiving bool
	done      bool
	err       error
}

func NewReceiveModel() ReceiveModel {
	ti := textinput.New()
	ti.Placeholder = "7-code-words"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40

	return ReceiveModel{
		textInput: ti,
		status:    "Enter Wormhole Code:",
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
				return m, startReceive(code)
			}
		case tea.KeyEsc:
			return m, tea.Quit
		}

	case HandshakeSuccessMsg:
		m.status = "Connected! Receiving..."
		return m, nil

	case TransferDoneMsg:
		m.done = true
		m.status = "Received File! (saved as 'received_file')"
		return m, tea.Quit

	case ErrorMsg:
		m.err = msg
		m.status = fmt.Sprintf("Error: %v", msg)
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m ReceiveModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress q to quit", m.err)
	}
	if m.done {
		return fmt.Sprintf("%s\n\nPress q to quit", m.status)
	}
	return fmt.Sprintf("%s\n\n%s", m.status, m.textInput.View())
}

func startReceive(code string) tea.Cmd {
	return func() tea.Msg {
		c := wormhole.NewClient("")
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

		// Receive file
		out, err := os.Create("received_file") // Simple default name
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
