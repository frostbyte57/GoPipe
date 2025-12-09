package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type State int

const (
	StateMenu State = iota
	StateSend
	StateReceive
)

type Model struct {
	state        State
	choices      []string
	cursor       int
	sendModel    SendModel
	receiveModel ReceiveModel
}

func InitialModel() Model {
	return Model{
		state:        StateMenu,
		choices:      []string{"Send File", "Receive File"},
		sendModel:    NewSendModel(),
		receiveModel: NewReceiveModel(),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global Quit
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// If in Menu
		if m.state == StateMenu {
			switch msg.String() {
			case "q":
				return m, tea.Quit
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
					return m, m.sendModel.Init()
				} else {
					m.state = StateReceive
					return m, m.receiveModel.Init()
				}
			}
		}
	}

	switch m.state {
	case StateSend:
		var startCmd tea.Cmd
		// We need to cast back to SendModel to reassign?
		// Bubble Tea models are value types.
		// m.sendModel.Update returns (tea.Model, tea.Cmd)
		newM, newCmd := m.sendModel.Update(msg)
		m.sendModel = newM.(SendModel)
		startCmd = newCmd

		// If send model quits, we could go back to menu?
		// For now sendModel sends tea.Quit when done.
		return m, startCmd

	case StateReceive:
		newM, newCmd := m.receiveModel.Update(msg)
		m.receiveModel = newM.(ReceiveModel)
		return m, newCmd
	}

	return m, nil
}

func (m Model) View() string {
	switch m.state {
	case StateMenu:
		s := "What would you like to do?\n\n"

		for i, choice := range m.choices {
			cursor := " " // no cursor
			if m.cursor == i {
				cursor = ">" // cursor!
			}

			s += fmt.Sprintf("%s %s\n", cursor, choice)
		}

		s += "\nPress q to quit.\n"
		return s
	case StateSend:
		return m.sendModel.View()
	case StateReceive:
		return m.receiveModel.View()
	}
	return ""
}
