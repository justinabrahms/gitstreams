package main

import (
	"bytes"
	"testing"
)

func TestGreeting(t *testing.T) {
	got := greeting()
	want := "Hello, gitstreams!"
	if got != want {
		t.Errorf("greeting() = %q, want %q", got, want)
	}
}

func TestRun(t *testing.T) {
	var buf bytes.Buffer
	run(&buf)
	got := buf.String()
	want := "Hello, gitstreams!\n"
	if got != want {
		t.Errorf("run() output = %q, want %q", got, want)
	}
}
