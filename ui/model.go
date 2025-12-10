package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type State int

const (
	StateMenu State = iota
	StateSend
	StateReceive
	StateSettings
)

type Model struct {
	state         State
	choices       []string
	cursor        int
	sendModel     SendModel
	receiveModel  ReceiveModel
	settingsModel SettingsModel
	confirmExit   bool
}

func InitialModel(mailboxURL string) Model {
	return Model{
		state:         StateMenu,
		choices:       []string{"Send File", "Receive File", "Settings"},
		sendModel:     NewSendModel(mailboxURL),
		receiveModel:  NewReceiveModel(mailboxURL),
		settingsModel: NewSettingsModel(),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global Key Handling
	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == "ctrl+c" {
			if m.confirmExit {
				return m, tea.Quit
			}
			m.confirmExit = true
			return m, nil // Trigger view update to show warning
		}
		// Reset valid flag on any other key
		m.confirmExit = false
	}

	// State-specific Handling
	switch m.state {
	case StateMenu:
		return m.updateMenu(msg)

	case StateSend:
		newM, cmd := m.sendModel.Update(msg)
		m.sendModel = newM.(SendModel)
		return m, cmd

	case StateReceive:
		newM, cmd := m.receiveModel.Update(msg)
		m.receiveModel = newM.(ReceiveModel)
		return m, cmd

	case StateSettings:
		newM, cmd := m.settingsModel.Update(msg)
		m.settingsModel = newM
		// Check for exit from settings
		if msg, ok := msg.(tea.KeyMsg); ok && msg.Type == tea.KeyEsc {
			m.state = StateMenu
		}
		return m, cmd
	}

	return m, nil
}

func (m Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter", " ":
			if m.cursor == 0 {
				m.state = StateSend
				m.sendModel = NewSendModel(m.sendModel.mailboxURL)
				return m, m.sendModel.Init()
			} else if m.cursor == 1 {
				m.state = StateReceive
				m.receiveModel = NewReceiveModel(m.receiveModel.mailboxURL)
				return m, m.receiveModel.Init()
			} else {
				m.state = StateSettings
				return m, nil
			}
		}

	case BackToMenuMsg:
		m.state = StateMenu
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	var content string
	switch m.state {
	case StateMenu:
		content = m.viewMenu()
	case StateSend:
		content = m.sendModel.View()
	case StateReceive:
		content = m.receiveModel.View()
	case StateSettings:
		content = m.settingsModel.View()
	}

	// Add Exit Confirmation Overlay/Append
	if m.confirmExit {
		content += "\n\n" + WarnStyle.Render("Press Ctrl+C again to exit")
	}

	return AppStyle.Render(content)
}

func (m Model) viewMenu() string {
	s := RenderLogo() + "\n\n"
	s += "What would you like to do?\n\n"

	for i, choice := range m.choices {
		cursor := "  "
		choiceStr := choice
		if m.cursor == i {
			cursor = lipgloss.NewStyle().Foreground(ColorPurple).Render("> ")
			choiceStr = lipgloss.NewStyle().Foreground(ColorGoBlue).Bold(true).Render(choice)
		}
		s += fmt.Sprintf("%s%s\n", cursor, choiceStr)
	}

	s += HelpStyle.Render("\nUse Up/Down to navigate, Enter to select.\n")
	return s
}
