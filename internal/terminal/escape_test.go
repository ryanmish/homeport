package terminal

import (
	"testing"
)

func TestExtractOSCTitle(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantTitle string
		wantFound bool
	}{
		{
			name:      "OSC 0 with BEL terminator",
			data:      []byte("\x1b]0;My Terminal Title\x07"),
			wantTitle: "My Terminal Title",
			wantFound: true,
		},
		{
			name:      "OSC 1 with BEL terminator (icon name)",
			data:      []byte("\x1b]1;Icon Name\x07"),
			wantTitle: "Icon Name",
			wantFound: true,
		},
		{
			name:      "OSC 2 with BEL terminator (window title)",
			data:      []byte("\x1b]2;Window Title\x07"),
			wantTitle: "Window Title",
			wantFound: true,
		},
		{
			name:      "OSC 0 with ST terminator (ESC backslash)",
			data:      []byte("\x1b]0;Title with ST\x1b\\"),
			wantTitle: "Title with ST",
			wantFound: true,
		},
		{
			name:      "OSC 2 with ST terminator",
			data:      []byte("\x1b]2;Another Title\x1b\\"),
			wantTitle: "Another Title",
			wantFound: true,
		},
		{
			name:      "title with surrounding text",
			data:      []byte("some output\x1b]0;Terminal Title\x07more output"),
			wantTitle: "Terminal Title",
			wantFound: true,
		},
		{
			name:      "title with special characters",
			data:      []byte("\x1b]0;~/projects/homeport - zsh\x07"),
			wantTitle: "~/projects/homeport - zsh",
			wantFound: true,
		},
		{
			name:      "title with unicode",
			data:      []byte("\x1b]0;Terminal ❯ homeport\x07"),
			wantTitle: "Terminal ❯ homeport",
			wantFound: true,
		},
		{
			name:      "empty title",
			data:      []byte("\x1b]0;\x07"),
			wantTitle: "",
			wantFound: true,
		},
		{
			name:      "no OSC sequence",
			data:      []byte("regular terminal output"),
			wantTitle: "",
			wantFound: false,
		},
		{
			name:      "OSC with non-title type (OSC 4 - color)",
			data:      []byte("\x1b]4;0;rgb:00/00/00\x07"),
			wantTitle: "",
			wantFound: false,
		},
		{
			name:      "incomplete OSC sequence (no terminator)",
			data:      []byte("\x1b]0;Incomplete Title"),
			wantTitle: "",
			wantFound: false,
		},
		{
			name:      "OSC without digit",
			data:      []byte("\x1b];Missing Digit\x07"),
			wantTitle: "",
			wantFound: false,
		},
		{
			name:      "OSC without semicolon",
			data:      []byte("\x1b]0Missing Semicolon\x07"),
			wantTitle: "",
			wantFound: false,
		},
		{
			name:      "just ESC bracket (no content)",
			data:      []byte("\x1b]"),
			wantTitle: "",
			wantFound: false,
		},
		{
			name:      "BEL terminator closer than ST",
			data:      []byte("\x1b]0;Title\x07extra\x1b\\"),
			wantTitle: "Title",
			wantFound: true,
		},
		{
			name:      "ST terminator closer than BEL",
			data:      []byte("\x1b]0;Title\x1b\\extra\x07"),
			wantTitle: "Title",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotFound := ExtractOSCTitle(tt.data)
			if gotTitle != tt.wantTitle {
				t.Errorf("ExtractOSCTitle() title = %q, want %q", gotTitle, tt.wantTitle)
			}
			if gotFound != tt.wantFound {
				t.Errorf("ExtractOSCTitle() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

func TestExtractAllOSCTitles(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantTitle string
		wantFound bool
	}{
		{
			name:      "single title",
			data:      []byte("\x1b]0;First Title\x07"),
			wantTitle: "First Title",
			wantFound: true,
		},
		{
			name:      "multiple titles - returns last",
			data:      []byte("\x1b]0;First\x07\x1b]0;Second\x07\x1b]0;Third\x07"),
			wantTitle: "Third",
			wantFound: true,
		},
		{
			name:      "multiple titles with mixed terminators",
			data:      []byte("\x1b]0;First\x07\x1b]2;Last Title\x1b\\"),
			wantTitle: "Last Title",
			wantFound: true,
		},
		{
			name:      "titles with output between them",
			data:      []byte("\x1b]0;Title One\x07some output\x1b]0;Title Two\x07more output"),
			wantTitle: "Title Two",
			wantFound: true,
		},
		{
			name:      "no titles",
			data:      []byte("just regular output"),
			wantTitle: "",
			wantFound: false,
		},
		{
			name:      "realistic shell prompt sequence",
			data:      []byte("\x1b]0;user@host: ~\x07\x1b]2;zsh\x07"),
			wantTitle: "zsh",
			wantFound: true,
		},
		{
			name:      "claude code style title updates",
			data:      []byte("\x1b]2;claude - init\x07working...\x1b]2;claude - ready\x07"),
			wantTitle: "claude - ready",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotFound := ExtractAllOSCTitles(tt.data)
			if gotTitle != tt.wantTitle {
				t.Errorf("ExtractAllOSCTitles() title = %q, want %q", gotTitle, tt.wantTitle)
			}
			if gotFound != tt.wantFound {
				t.Errorf("ExtractAllOSCTitles() found = %v, want %v", gotFound, tt.wantFound)
			}
		})
	}
}

func TestAlternateScreenSequences(t *testing.T) {
	// Test that the alternate screen sequences are correctly defined
	// These are used to detect when TUI apps (vim, less, etc.) are active

	tests := []struct {
		name    string
		data    []byte
		isEnter bool
		isExit  bool
	}{
		{
			name:    "xterm enter alternate screen",
			data:    []byte("\x1b[?1049h"),
			isEnter: true,
			isExit:  false,
		},
		{
			name:    "xterm exit alternate screen",
			data:    []byte("\x1b[?1049l"),
			isEnter: false,
			isExit:  true,
		},
		{
			name:    "legacy enter alternate screen",
			data:    []byte("\x1b[?47h"),
			isEnter: true,
			isExit:  false,
		},
		{
			name:    "legacy exit alternate screen",
			data:    []byte("\x1b[?47l"),
			isEnter: false,
			isExit:  true,
		},
		{
			name:    "variant enter alternate screen",
			data:    []byte("\x1b[?1047h"),
			isEnter: true,
			isExit:  false,
		},
		{
			name:    "variant exit alternate screen",
			data:    []byte("\x1b[?1047l"),
			isEnter: false,
			isExit:  true,
		},
		{
			name:    "regular output",
			data:    []byte("hello world"),
			isEnter: false,
			isExit:  false,
		},
		{
			name:    "alternate screen enter embedded in output",
			data:    []byte("starting vim\x1b[?1049heditor content"),
			isEnter: true,
			isExit:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEnter := containsAnySequence(tt.data, altScreenEnter)
			gotExit := containsAnySequence(tt.data, altScreenExit)

			if gotEnter != tt.isEnter {
				t.Errorf("alternate screen enter detection = %v, want %v", gotEnter, tt.isEnter)
			}
			if gotExit != tt.isExit {
				t.Errorf("alternate screen exit detection = %v, want %v", gotExit, tt.isExit)
			}
		})
	}
}

// containsAnySequence checks if data contains any of the given sequences
func containsAnySequence(data []byte, sequences [][]byte) bool {
	for _, seq := range sequences {
		if containsSequence(data, seq) {
			return true
		}
	}
	return false
}

func containsSequence(data, seq []byte) bool {
	if len(seq) > len(data) {
		return false
	}
	for i := 0; i <= len(data)-len(seq); i++ {
		match := true
		for j := 0; j < len(seq); j++ {
			if data[i+j] != seq[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
