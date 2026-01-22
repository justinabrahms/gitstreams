// Package progress provides progress indicators for CLI output.
package progress

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

// Spinner displays a spinning progress indicator with a message.
type Spinner struct {
	w        io.Writer
	done     chan struct{}
	message  string
	frames   []string
	interval time.Duration
	mu       sync.Mutex
	active   bool
	isTTY    bool
}

// NewSpinner creates a new spinner that writes to the given writer.
// If the writer is a terminal, it will show an animated spinner.
// Otherwise, it will print static progress messages.
func NewSpinner(w io.Writer) *Spinner {
	isTTY := false
	if f, ok := w.(interface{ Fd() uintptr }); ok {
		isTTY = isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}

	return &Spinner{
		w:        w,
		interval: 100 * time.Millisecond,
		frames:   []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		isTTY:    isTTY,
	}
}

// Start begins the spinner animation with the given message.
func (s *Spinner) Start(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		return
	}

	s.message = message
	s.active = true
	s.done = make(chan struct{})

	if s.isTTY {
		go s.spin()
	} else {
		// Non-TTY: just print the message
		_, _ = fmt.Fprintf(s.w, "%s\n", message)
	}
}

// Update changes the spinner message without stopping.
func (s *Spinner) Update(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.message = message

	if !s.isTTY && s.active {
		// Non-TTY: print each update on a new line
		_, _ = fmt.Fprintf(s.w, "%s\n", message)
	}
}

// Stop stops the spinner and clears the line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}

	s.active = false
	close(s.done)

	if s.isTTY {
		// Clear the spinner line
		_, _ = fmt.Fprintf(s.w, "\r%s\r", strings.Repeat(" ", len(s.message)+4))
	}
}

// StopWithMessage stops the spinner and displays a final message.
func (s *Spinner) StopWithMessage(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		if message != "" {
			_, _ = fmt.Fprintf(s.w, "%s\n", message)
		}
		return
	}

	s.active = false
	close(s.done)

	if s.isTTY {
		// Clear and print final message
		_, _ = fmt.Fprintf(s.w, "\r%s\r", strings.Repeat(" ", len(s.message)+4))
	}
	if message != "" {
		_, _ = fmt.Fprintf(s.w, "%s\n", message)
	}
}

func (s *Spinner) spin() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	frame := 0
	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			msg := s.message
			s.mu.Unlock()

			_, _ = fmt.Fprintf(s.w, "\r%s %s", s.frames[frame], msg)
			frame = (frame + 1) % len(s.frames)
		}
	}
}

// Progress tracks progress through a set of items.
type Progress struct {
	w       io.Writer
	spinner *Spinner
	total   int
	current int
	mu      sync.Mutex
}

// NewProgress creates a new progress tracker.
func NewProgress(w io.Writer, total int) *Progress {
	return &Progress{
		w:       w,
		spinner: NewSpinner(w),
		total:   total,
	}
}

// Start begins progress tracking with an initial message.
func (p *Progress) Start(message string) {
	p.spinner.Start(message)
}

// SetItem updates progress to show the current item being processed.
// Format: "Fetching activity for user 3/47: torvalds..."
func (p *Progress) SetItem(current int, itemName string) {
	p.mu.Lock()
	p.current = current
	p.mu.Unlock()

	msg := fmt.Sprintf("Fetching activity for user %d/%d: %s...", current, p.total, itemName)
	p.spinner.Update(msg)
}

// Done stops the progress indicator.
func (p *Progress) Done() {
	p.spinner.Stop()
}

// DoneWithMessage stops the progress indicator and shows a message.
func (p *Progress) DoneWithMessage(message string) {
	p.spinner.StopWithMessage(message)
}
