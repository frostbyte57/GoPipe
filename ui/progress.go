package ui

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// ProgressReader wraps an io.Reader and sends tea.Msg for progress updates.
type ProgressReader struct {
	Reader  io.Reader
	Total   int64
	Current int64
	Program *tea.Program // We need to send messages to the program
	// BUT, we can't safely access Program from a goroutine if we are IN the update loop logic usually?
	// Actually, tea.Program.Send() is thread-safe.
	// Alternatively, we can use a channel if we run this in a command.
	// Better yet: passing a callback channel.

	OnProgress func(float64)
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	if n > 0 {
		pr.Current += int64(n)
		if pr.OnProgress != nil && pr.Total > 0 {
			ratio := float64(pr.Current) / float64(pr.Total)
			pr.OnProgress(ratio)
		}
	}
	return n, err
}
