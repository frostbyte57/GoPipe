package ui

import "github.com/charmbracelet/lipgloss"

// GoPipe ASCII Art
// We can use a raw string.
// Standard font or something slanted.
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
	// Simple gradient effect by coloring lines?
	// Or we can just make it one solid Gradient if lipgloss supported it easily (it does have simple support).

	// Let's make it Go Blue fading to Purple?
	// Lipgloss doesn't have built-in meaningful text gradients without helper libs,
	// so we'll just style it solid or construct it.

	// Solid Go Blue for now, maybe with a Purple shadow/underline?
	return lipgloss.NewStyle().
		Foreground(ColorGoBlue).
		Bold(true).
		Render(LogoASCII)
}
