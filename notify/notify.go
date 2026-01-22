// Package notify provides desktop notification support for macOS.
package notify

import (
	"fmt"
	"os/exec"
)

// Notification represents a desktop notification.
type Notification struct {
	Title    string
	Message  string
	Subtitle string
	Sound    string // macOS sound name (e.g., "default", "Ping", "Basso")
	OpenURL  string // URL to open when notification is clicked (terminal-notifier only)
}

// Notifier sends desktop notifications.
type Notifier interface {
	Send(n Notification) error
}

// CommandExecutor runs shell commands. Used for dependency injection in tests.
type CommandExecutor interface {
	LookPath(file string) (string, error)
	Run(name string, args ...string) error
}

// DefaultExecutor implements CommandExecutor using os/exec.
type DefaultExecutor struct{}

// LookPath searches for an executable in PATH.
func (DefaultExecutor) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// Run executes a command.
func (DefaultExecutor) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

// MacNotifier sends notifications on macOS using terminal-notifier or osascript.
type MacNotifier struct {
	Executor CommandExecutor
}

// NewMacNotifier creates a new MacNotifier with the default executor.
func NewMacNotifier() *MacNotifier {
	return &MacNotifier{Executor: DefaultExecutor{}}
}

// Send sends a notification using terminal-notifier if available, otherwise osascript.
func (m *MacNotifier) Send(n Notification) error {
	if n.Message == "" {
		return fmt.Errorf("notification message cannot be empty")
	}

	// Try terminal-notifier first (better UX, more features)
	if _, err := m.Executor.LookPath("terminal-notifier"); err == nil {
		return m.sendTerminalNotifier(n)
	}

	// Fall back to osascript (always available on macOS)
	return m.sendOsascript(n)
}

// sendTerminalNotifier sends a notification using terminal-notifier.
func (m *MacNotifier) sendTerminalNotifier(n Notification) error {
	args := []string{"-message", n.Message}

	if n.Title != "" {
		args = append(args, "-title", n.Title)
	}
	if n.Subtitle != "" {
		args = append(args, "-subtitle", n.Subtitle)
	}
	if n.Sound != "" {
		args = append(args, "-sound", n.Sound)
	}
	if n.OpenURL != "" {
		args = append(args, "-open", n.OpenURL)
	}

	return m.Executor.Run("terminal-notifier", args...)
}

// sendOsascript sends a notification using osascript.
// Note: osascript's display notification does not support click actions.
// For click-to-open functionality, install terminal-notifier: brew install terminal-notifier
func (m *MacNotifier) sendOsascript(n Notification) error {
	// Build AppleScript for display notification
	// OpenURL is intentionally ignored as osascript does not support click actions
	script := fmt.Sprintf(`display notification %q`, n.Message)

	if n.Title != "" {
		script += fmt.Sprintf(` with title %q`, n.Title)
	}
	if n.Subtitle != "" {
		script += fmt.Sprintf(` subtitle %q`, n.Subtitle)
	}
	if n.Sound != "" {
		script += fmt.Sprintf(` sound name %q`, n.Sound)
	}

	return m.Executor.Run("osascript", "-e", script)
}
