package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	// Go Blue: #00ADD8
	// Purple: #9D7CD8 (Soft Purple) or #7D56F4 (Vibrant)
	ColorGoBlue     = lipgloss.Color("#00ADD8")
	ColorPurple     = lipgloss.Color("#7D56F4")
	ColorSubtle     = lipgloss.Color("#626262")
	ColorBackground = lipgloss.Color("#1C1B1F")
	ColorSurface    = lipgloss.Color("#49454F")
	ColorSuccess    = lipgloss.Color("#00ADD8") // Success is also Blue (Go style) instead of Green? Or keep Green.
	// Let's keep Success Green for semantics, or Cyanish.
	ColorText  = lipgloss.Color("#FFFFFF")
	ColorError = lipgloss.Color("#FF5F87")

	// Text Styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorGoBlue).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorPurple).
			Italic(true)

	StatusStyle = lipgloss.NewStyle().
			Foreground(ColorSubtle)

	// Code Box Style
	CodeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPurple).
			Padding(1, 4).
			Margin(1, 0).
			Align(lipgloss.Center).
			Foreground(ColorGoBlue).
			Bold(true)

	// Help Style
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorSubtle).
			MarginTop(1)

	// Input Style
	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSurface).
			Padding(0, 1)

	FocusedInputStyle = InputStyle.Copy().
				BorderForeground(ColorGoBlue)

	// Logo Style for Gradient (simulation via separate chars or block)
	LogoStyle = lipgloss.NewStyle().
			Bold(true).
			MarginBottom(1)

	// Main Application Container
	AppStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGoBlue).
			Padding(1, 2).
			Margin(1, 1).
			Width(60) // Fixed width for cleaner look? Or adaptive?

	WarnStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)
)
