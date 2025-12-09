package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/frostbyte57/GoPipe/internal/config"
)

type SettingsModel struct {
	textInput textinput.Model
	status    string
	cfg       *config.Config
}

func NewSettingsModel() SettingsModel {
	ti := textinput.New()
	ti.Placeholder = "/path/to/download/dir"
	ti.Focus()
	ti.Width = 40
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorText)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ColorGoBlue)

	cfg, _ := config.LoadConfig() // Ignore error, use default
	if cfg != nil {
		ti.SetValue(cfg.DownloadDir)
	}

	return SettingsModel{
		textInput: ti,
		status:    "Edit Download Directory:",
		cfg:       cfg,
	}
}

func (m SettingsModel) Update(msg tea.Msg) (SettingsModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			// Save
			m.cfg.DownloadDir = m.textInput.Value()
			if err := config.SaveConfig(m.cfg); err != nil {
				m.status = fmt.Sprintf("Error saving: %v", err)
			} else {
				m.status = "Settings Saved!"
			}
			return m, nil // OR return to menu logic?
		case tea.KeyEsc:
			return m, nil // handled by parent to go back
		}
	}
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m SettingsModel) View() string {
	return fmt.Sprintf("\n%s\n\n%s\n\n%s\n%s",
		TitleStyle.Render("Settings"),
		m.status,
		m.textInput.View(),
		HelpStyle.Render("Press Enter to Save, Esc to Return"),
	)
}
