package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/frostbyte57/GoPipe/internal/mailbox"
	"github.com/frostbyte57/GoPipe/ui"
)

func main() {
	mailboxURL := flag.String("mailbox", mailbox.DefaultURL, "WebSocket URL of the Mailbox Server")
	flag.Parse()

	p := tea.NewProgram(ui.InitialModel(*mailboxURL))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
