package terminal

import (
	"bytes"
	"time"
)

// MinCommandDuration is the minimum duration for a command to trigger a completion event.
// Quick commands like 'ls', 'cd' won't trigger notifications.
const MinCommandDuration = 3 * time.Second

// Common shell prompt patterns (at end of line or data)
var promptPatterns = [][]byte{
	[]byte("$ "),  // bash default
	[]byte("# "),  // root prompt
	[]byte("% "),  // zsh/csh
	[]byte("> "),  // generic / continuation
	[]byte("❯ "), // starship/powerline
	[]byte("➜ "), // oh-my-zsh
}

// CommandCompletion represents a completed command
type CommandCompletion struct {
	Duration time.Duration
}

// CommandTracker tracks command execution state for a terminal session.
// It detects when commands complete by watching for shell prompts.
type CommandTracker struct {
	lastPromptTime time.Time
	commandRunning bool
	commandStart   time.Time
}

// NewCommandTracker creates a new CommandTracker
func NewCommandTracker() *CommandTracker {
	return &CommandTracker{
		lastPromptTime: time.Now(),
	}
}

// ProcessOutput analyzes terminal output and returns a CommandCompletion
// if a command appears to have finished.
func (t *CommandTracker) ProcessOutput(data []byte) *CommandCompletion {
	// Check if output contains a prompt pattern
	hasPrompt := false
	for _, pattern := range promptPatterns {
		if bytes.Contains(data, pattern) {
			hasPrompt = true
			break
		}
	}

	if hasPrompt {
		var completion *CommandCompletion

		// If a command was running, it just finished
		if t.commandRunning {
			duration := time.Since(t.commandStart)
			if duration >= MinCommandDuration {
				completion = &CommandCompletion{Duration: duration}
			}
			t.commandRunning = false
		}

		t.lastPromptTime = time.Now()
		return completion
	}

	// If we see output and weren't already tracking a command, start tracking
	// This is heuristic: output after a prompt usually means a command is running
	if !t.commandRunning && len(data) > 0 {
		// Only consider it a command start if we recently saw a prompt
		// and this output contains something meaningful (not just whitespace)
		if time.Since(t.lastPromptTime) < 500*time.Millisecond && hasNonWhitespace(data) {
			t.commandRunning = true
			t.commandStart = time.Now()
		}
	}

	return nil
}

// hasNonWhitespace checks if data contains any non-whitespace characters
func hasNonWhitespace(data []byte) bool {
	for _, b := range data {
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			return true
		}
	}
	return false
}

// Reset resets the command tracker state
func (t *CommandTracker) Reset() {
	t.commandRunning = false
	t.lastPromptTime = time.Now()
}

// IsCommandRunning returns whether a command appears to be running
func (t *CommandTracker) IsCommandRunning() bool {
	return t.commandRunning
}
