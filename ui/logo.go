package ui

import "github.com/charmbracelet/lipgloss"

/*
   ______      ____  _
  / ____/___  / __ \(_)___  ___
 / / __/ __ \/ /_/ / / __ \/ _ \
/ /_/ / /_/ / ____/ / /_/ /  __/
\____/\____/_/   /_/ .___/\___/
                  /_/
*/

const LogoASCII = `
   ______      ____  _
  / ____/___  / __ \(_)___  ___
 / / __/ __ \/ /_/ / / __ \/ _ \
/ /_/ / /_/ / ____/ / /_/ /  __/
\____/\____/_/   /_/ .___/\___/
                  /_/
`

func RenderLogo() string {
	return lipgloss.NewStyle().
		Foreground(ColorGoBlue).
		Bold(true).
		Render(LogoASCII)
}
