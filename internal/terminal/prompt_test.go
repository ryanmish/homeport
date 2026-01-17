package terminal

import (
	"testing"
	"time"
)

func TestNewCommandTracker(t *testing.T) {
	tracker := NewCommandTracker()
	if tracker == nil {
		t.Fatal("NewCommandTracker() returned nil")
	}
	if tracker.commandRunning {
		t.Error("new tracker should not have command running")
	}
	if tracker.lastPromptTime.IsZero() {
		t.Error("new tracker should have lastPromptTime set")
	}
}

func TestCommandTrackerReset(t *testing.T) {
	tracker := NewCommandTracker()
	tracker.commandRunning = true
	tracker.commandStart = time.Now().Add(-time.Minute)

	tracker.Reset()

	if tracker.commandRunning {
		t.Error("Reset() should clear commandRunning")
	}
	if time.Since(tracker.lastPromptTime) > time.Second {
		t.Error("Reset() should update lastPromptTime to now")
	}
}

func TestCommandTrackerIsCommandRunning(t *testing.T) {
	tracker := NewCommandTracker()

	if tracker.IsCommandRunning() {
		t.Error("new tracker should not have command running")
	}

	tracker.commandRunning = true
	if !tracker.IsCommandRunning() {
		t.Error("tracker.IsCommandRunning() should return true when command is running")
	}
}

func TestHasNonWhitespace(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{
			name: "empty",
			data: []byte{},
			want: false,
		},
		{
			name: "spaces only",
			data: []byte("   "),
			want: false,
		},
		{
			name: "tabs only",
			data: []byte("\t\t"),
			want: false,
		},
		{
			name: "newlines only",
			data: []byte("\n\n\r\n"),
			want: false,
		},
		{
			name: "mixed whitespace",
			data: []byte(" \t\n\r "),
			want: false,
		},
		{
			name: "single character",
			data: []byte("a"),
			want: true,
		},
		{
			name: "text with whitespace",
			data: []byte("  hello  "),
			want: true,
		},
		{
			name: "command output",
			data: []byte("ls -la\n"),
			want: true,
		},
		{
			name: "escape sequence",
			data: []byte("\x1b[32m"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasNonWhitespace(tt.data)
			if got != tt.want {
				t.Errorf("hasNonWhitespace(%q) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestPromptPatterns(t *testing.T) {
	// Verify that all expected prompt patterns are defined
	expectedPatterns := []string{
		"$ ", // bash default
		"# ", // root prompt
		"% ", // zsh/csh
		"> ", // generic
		"❯ ", // starship/powerline
		"➜ ", // oh-my-zsh
	}

	for _, expected := range expectedPatterns {
		found := false
		for _, pattern := range promptPatterns {
			if string(pattern) == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected prompt pattern %q not found in promptPatterns", expected)
		}
	}
}

func TestProcessOutputPromptDetection(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		hasPrompt bool
	}{
		{
			name:      "bash prompt",
			data:      []byte("user@host:~$ "),
			hasPrompt: true,
		},
		{
			name:      "root prompt",
			data:      []byte("root@host:~# "),
			hasPrompt: true,
		},
		{
			name:      "zsh prompt",
			data:      []byte("user@host ~ % "),
			hasPrompt: true,
		},
		{
			name:      "generic prompt",
			data:      []byte("> "),
			hasPrompt: true,
		},
		{
			name:      "starship prompt",
			data:      []byte("~/projects ❯ "),
			hasPrompt: true,
		},
		{
			name:      "oh-my-zsh prompt",
			data:      []byte("➜ homeport "),
			hasPrompt: true,
		},
		{
			name:      "command output (no prompt)",
			data:      []byte("drwxr-xr-x  5 user user 4096 Jan  1 12:00 src"),
			hasPrompt: false,
		},
		{
			name:      "empty data",
			data:      []byte{},
			hasPrompt: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewCommandTracker()
			// First, simulate seeing a prompt
			tracker.ProcessOutput([]byte("$ "))
			// Wait a tiny bit
			time.Sleep(10 * time.Millisecond)

			// Now simulate command output
			tracker.ProcessOutput([]byte("some command\n"))

			// Check prompt detection in the given data
			gotPrompt := false
			for _, pattern := range promptPatterns {
				if containsSequence(tt.data, pattern) {
					gotPrompt = true
					break
				}
			}

			if gotPrompt != tt.hasPrompt {
				t.Errorf("prompt detection for %q = %v, want %v", tt.data, gotPrompt, tt.hasPrompt)
			}
		})
	}
}

func TestCommandTrackerNoCompletionForQuickCommands(t *testing.T) {
	tracker := NewCommandTracker()

	// Simulate seeing a prompt
	completion := tracker.ProcessOutput([]byte("$ "))
	if completion != nil {
		t.Error("should not have completion on initial prompt")
	}

	// Wait just a tiny bit (well under MinCommandDuration)
	time.Sleep(10 * time.Millisecond)

	// Simulate command starting
	tracker.ProcessOutput([]byte("ls\n"))

	// Immediately show new prompt (quick command)
	completion = tracker.ProcessOutput([]byte("file1 file2\n$ "))

	// Should NOT have a completion since command was too fast
	if completion != nil {
		t.Error("should not report completion for quick commands under MinCommandDuration")
	}
}

func TestCommandTrackerStateTransitions(t *testing.T) {
	tracker := NewCommandTracker()

	// Initial state: no command running
	if tracker.IsCommandRunning() {
		t.Error("initial state should have no command running")
	}

	// See a prompt
	tracker.ProcessOutput([]byte("$ "))
	time.Sleep(10 * time.Millisecond)

	// See non-whitespace output after prompt -> command starts
	tracker.ProcessOutput([]byte("npm install"))

	if !tracker.IsCommandRunning() {
		t.Error("after output following prompt, command should be running")
	}

	// See another prompt -> command ends
	tracker.ProcessOutput([]byte("\npackages installed\n$ "))

	if tracker.IsCommandRunning() {
		t.Error("after seeing prompt, command should no longer be running")
	}
}

func TestMinCommandDuration(t *testing.T) {
	// Verify the constant is set to a reasonable value
	if MinCommandDuration != 3*time.Second {
		t.Errorf("MinCommandDuration = %v, expected 3 seconds", MinCommandDuration)
	}
}

func TestCommandCompletionStruct(t *testing.T) {
	completion := &CommandCompletion{
		Duration: 5 * time.Second,
	}

	if completion.Duration != 5*time.Second {
		t.Errorf("CommandCompletion.Duration = %v, want 5s", completion.Duration)
	}
}
