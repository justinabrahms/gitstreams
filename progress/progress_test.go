package progress

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestSpinner_NonTTY(t *testing.T) {
	var buf bytes.Buffer

	s := NewSpinner(&buf)

	// Should not be TTY since bytes.Buffer doesn't have Fd()
	if s.isTTY {
		t.Error("expected non-TTY for bytes.Buffer")
	}

	s.Start("Loading...")
	time.Sleep(10 * time.Millisecond) // Small delay to ensure goroutine could start

	output := buf.String()
	if !strings.Contains(output, "Loading...") {
		t.Errorf("expected 'Loading...' in output, got: %q", output)
	}

	s.Stop()
}

func TestSpinner_Update_NonTTY(t *testing.T) {
	var buf bytes.Buffer

	s := NewSpinner(&buf)
	s.Start("Step 1")
	s.Update("Step 2")
	s.Update("Step 3")
	s.Stop()

	output := buf.String()
	if !strings.Contains(output, "Step 1") {
		t.Errorf("expected 'Step 1' in output, got: %q", output)
	}
	if !strings.Contains(output, "Step 2") {
		t.Errorf("expected 'Step 2' in output, got: %q", output)
	}
	if !strings.Contains(output, "Step 3") {
		t.Errorf("expected 'Step 3' in output, got: %q", output)
	}
}

func TestSpinner_StopWithMessage(t *testing.T) {
	var buf bytes.Buffer

	s := NewSpinner(&buf)
	s.Start("Loading...")
	s.StopWithMessage("Done!")

	output := buf.String()
	if !strings.Contains(output, "Done!") {
		t.Errorf("expected 'Done!' in output, got: %q", output)
	}
}

func TestSpinner_StopWithMessage_NotStarted(t *testing.T) {
	var buf bytes.Buffer

	s := NewSpinner(&buf)
	s.StopWithMessage("Done!")

	output := buf.String()
	if !strings.Contains(output, "Done!") {
		t.Errorf("expected 'Done!' in output, got: %q", output)
	}
}

func TestProgress_SetItem(t *testing.T) {
	var buf bytes.Buffer

	p := NewProgress(&buf, 5)
	p.Start("Starting...")
	p.SetItem(1, "user1")
	p.SetItem(2, "user2")
	p.SetItem(3, "user3")
	p.Done()

	output := buf.String()
	if !strings.Contains(output, "Fetching activity for user 1/5: user1...") {
		t.Errorf("expected progress message for user1, got: %q", output)
	}
	if !strings.Contains(output, "Fetching activity for user 2/5: user2...") {
		t.Errorf("expected progress message for user2, got: %q", output)
	}
	if !strings.Contains(output, "Fetching activity for user 3/5: user3...") {
		t.Errorf("expected progress message for user3, got: %q", output)
	}
}

func TestProgress_DoneWithMessage(t *testing.T) {
	var buf bytes.Buffer

	p := NewProgress(&buf, 3)
	p.Start("Starting...")
	p.SetItem(1, "user1")
	p.DoneWithMessage("All done!")

	output := buf.String()
	if !strings.Contains(output, "All done!") {
		t.Errorf("expected 'All done!' in output, got: %q", output)
	}
}

func TestSpinner_MultipleStops(t *testing.T) {
	var buf bytes.Buffer

	s := NewSpinner(&buf)
	s.Start("Loading...")
	s.Stop()
	s.Stop() // Second stop should be a no-op

	// Should not panic
}

func TestSpinner_MultipleStarts(t *testing.T) {
	var buf bytes.Buffer

	s := NewSpinner(&buf)
	s.Start("First")
	s.Start("Second") // Should be ignored
	s.Stop()

	output := buf.String()
	if !strings.Contains(output, "First") {
		t.Errorf("expected 'First' in output, got: %q", output)
	}
	// "Second" should not appear because start was ignored
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > 1 {
		t.Errorf("expected only one line of output, got %d: %q", len(lines), output)
	}
}
