package notify

import (
	"errors"
	"reflect"
	"testing"
)

// mockExecutor records calls and controls return values for testing.
//
//nolint:govet // field alignment not critical for test struct
type mockExecutor struct {
	lookPathResults map[string]error
	runCalls        []runCall
	runError        error
}

type runCall struct {
	Name string
	Args []string
}

func (m *mockExecutor) LookPath(file string) (string, error) {
	if err, ok := m.lookPathResults[file]; ok {
		if err != nil {
			return "", err
		}
		return "/usr/local/bin/" + file, nil
	}
	return "", errors.New("not found")
}

func (m *mockExecutor) Run(name string, args ...string) error {
	m.runCalls = append(m.runCalls, runCall{Name: name, Args: args})
	return m.runError
}

func TestMacNotifier_Send_TerminalNotifier(t *testing.T) {
	mock := &mockExecutor{
		lookPathResults: map[string]error{
			"terminal-notifier": nil, // available
		},
	}

	notifier := &MacNotifier{Executor: mock}

	tests := []struct {
		name     string
		notif    Notification
		wantArgs []string
	}{
		{
			name:     "message only",
			notif:    Notification{Message: "Hello"},
			wantArgs: []string{"-message", "Hello"},
		},
		{
			name:     "with title",
			notif:    Notification{Title: "Test", Message: "Hello"},
			wantArgs: []string{"-message", "Hello", "-title", "Test"},
		},
		{
			name:     "with subtitle",
			notif:    Notification{Title: "Test", Message: "Hello", Subtitle: "Sub"},
			wantArgs: []string{"-message", "Hello", "-title", "Test", "-subtitle", "Sub"},
		},
		{
			name:     "with sound",
			notif:    Notification{Title: "Test", Message: "Hello", Sound: "Ping"},
			wantArgs: []string{"-message", "Hello", "-title", "Test", "-sound", "Ping"},
		},
		{
			name:     "all fields",
			notif:    Notification{Title: "Test", Message: "Hello", Subtitle: "Sub", Sound: "default"},
			wantArgs: []string{"-message", "Hello", "-title", "Test", "-subtitle", "Sub", "-sound", "default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.runCalls = nil // reset

			err := notifier.Send(tt.notif)
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}

			if len(mock.runCalls) != 1 {
				t.Fatalf("expected 1 run call, got %d", len(mock.runCalls))
			}

			call := mock.runCalls[0]
			if call.Name != "terminal-notifier" {
				t.Errorf("expected terminal-notifier, got %s", call.Name)
			}
			if !reflect.DeepEqual(call.Args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", call.Args, tt.wantArgs)
			}
		})
	}
}

func TestMacNotifier_Send_Osascript(t *testing.T) {
	mock := &mockExecutor{
		lookPathResults: map[string]error{
			"terminal-notifier": errors.New("not found"),
		},
	}

	notifier := &MacNotifier{Executor: mock}

	tests := []struct {
		name       string
		notif      Notification
		wantScript string
	}{
		{
			name:       "message only",
			notif:      Notification{Message: "Hello"},
			wantScript: `display notification "Hello"`,
		},
		{
			name:       "with title",
			notif:      Notification{Title: "Test", Message: "Hello"},
			wantScript: `display notification "Hello" with title "Test"`,
		},
		{
			name:       "with subtitle",
			notif:      Notification{Title: "Test", Message: "Hello", Subtitle: "Sub"},
			wantScript: `display notification "Hello" with title "Test" subtitle "Sub"`,
		},
		{
			name:       "with sound",
			notif:      Notification{Title: "Test", Message: "Hello", Sound: "Ping"},
			wantScript: `display notification "Hello" with title "Test" sound name "Ping"`,
		},
		{
			name:       "all fields",
			notif:      Notification{Title: "Test", Message: "Hello", Subtitle: "Sub", Sound: "default"},
			wantScript: `display notification "Hello" with title "Test" subtitle "Sub" sound name "default"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.runCalls = nil // reset

			err := notifier.Send(tt.notif)
			if err != nil {
				t.Fatalf("Send() error = %v", err)
			}

			if len(mock.runCalls) != 1 {
				t.Fatalf("expected 1 run call, got %d", len(mock.runCalls))
			}

			call := mock.runCalls[0]
			if call.Name != "osascript" {
				t.Errorf("expected osascript, got %s", call.Name)
			}
			if len(call.Args) != 2 || call.Args[0] != "-e" {
				t.Errorf("expected [-e script], got %v", call.Args)
			}
			if call.Args[1] != tt.wantScript {
				t.Errorf("script = %q, want %q", call.Args[1], tt.wantScript)
			}
		})
	}
}

func TestMacNotifier_Send_EmptyMessage(t *testing.T) {
	mock := &mockExecutor{
		lookPathResults: map[string]error{
			"terminal-notifier": nil,
		},
	}

	notifier := &MacNotifier{Executor: mock}

	err := notifier.Send(Notification{Title: "Test"})
	if err == nil {
		t.Error("expected error for empty message, got nil")
	}
}

func TestMacNotifier_Send_RunError(t *testing.T) {
	expectedErr := errors.New("command failed")
	mock := &mockExecutor{
		lookPathResults: map[string]error{
			"terminal-notifier": nil,
		},
		runError: expectedErr,
	}

	notifier := &MacNotifier{Executor: mock}

	err := notifier.Send(Notification{Message: "Hello"})
	if err != expectedErr {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestNewMacNotifier(t *testing.T) {
	notifier := NewMacNotifier()

	// Use require-style pattern: fail fast if nil
	if notifier == nil || notifier.Executor == nil {
		t.Fatal("NewMacNotifier() returned nil or has nil Executor")
	}

	// Verify it's a DefaultExecutor
	if _, ok := notifier.Executor.(DefaultExecutor); !ok {
		t.Error("Executor is not DefaultExecutor")
	}
}

func TestDefaultExecutor_LookPath(t *testing.T) {
	exec := DefaultExecutor{}

	// Test with a command that should exist on macOS
	path, err := exec.LookPath("ls")
	if err != nil {
		t.Errorf("LookPath(ls) error = %v", err)
	}
	if path == "" {
		t.Error("LookPath(ls) returned empty path")
	}

	// Test with a command that shouldn't exist
	_, err = exec.LookPath("nonexistent-command-12345")
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

func TestNotifier_Interface(t *testing.T) {
	// Verify MacNotifier implements Notifier interface
	var _ Notifier = (*MacNotifier)(nil)
}
