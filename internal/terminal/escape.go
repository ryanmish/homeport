package terminal

import "bytes"

// OSC (Operating System Command) escape sequences for terminal titles:
// ESC ] 0 ; <title> BEL      - Set icon name and window title
// ESC ] 1 ; <title> BEL      - Set icon name
// ESC ] 2 ; <title> BEL      - Set window title
// ESC ] 0 ; <title> ESC \    - Alternative terminator (ST)

var (
	oscStart = []byte("\x1b]")    // ESC ]
	bel      = byte('\x07')       // BEL terminator
	st       = []byte("\x1b\\")   // String Terminator (ESC \)
)

// ExtractOSCTitle scans data for OSC title sequences and returns the title if found.
// It looks for OSC 0, 1, or 2 sequences which set the terminal title.
func ExtractOSCTitle(data []byte) (string, bool) {
	// Find OSC start
	idx := bytes.Index(data, oscStart)
	if idx == -1 {
		return "", false
	}

	// Move past ESC ]
	remaining := data[idx+2:]
	if len(remaining) == 0 {
		return "", false
	}

	// Check for OSC 0, 1, or 2 (title-related)
	if remaining[0] != '0' && remaining[0] != '1' && remaining[0] != '2' {
		return "", false
	}

	// Move past the digit
	remaining = remaining[1:]
	if len(remaining) == 0 {
		return "", false
	}

	// Expect semicolon
	if remaining[0] != ';' {
		return "", false
	}
	remaining = remaining[1:]

	// Find terminator (BEL or ST)
	belIdx := bytes.IndexByte(remaining, bel)
	stIdx := bytes.Index(remaining, st)

	var endIdx int
	if belIdx != -1 && (stIdx == -1 || belIdx < stIdx) {
		endIdx = belIdx
	} else if stIdx != -1 {
		endIdx = stIdx
	} else {
		// No terminator found - incomplete sequence
		return "", false
	}

	title := string(remaining[:endIdx])
	return title, true
}

// ExtractAllOSCTitles finds all OSC title sequences in data and returns the last one.
// This is useful because multiple title updates might be in a single data chunk.
func ExtractAllOSCTitles(data []byte) (string, bool) {
	var lastTitle string
	found := false

	remaining := data
	for {
		title, ok := ExtractOSCTitle(remaining)
		if !ok {
			break
		}
		lastTitle = title
		found = true

		// Find where this OSC sequence ended and continue searching
		idx := bytes.Index(remaining, oscStart)
		if idx == -1 {
			break
		}
		// Skip past this sequence
		remaining = remaining[idx+2:]
		// Find the terminator
		belIdx := bytes.IndexByte(remaining, bel)
		stIdx := bytes.Index(remaining, st)
		if belIdx != -1 && (stIdx == -1 || belIdx < stIdx) {
			remaining = remaining[belIdx+1:]
		} else if stIdx != -1 {
			remaining = remaining[stIdx+2:]
		} else {
			break
		}
	}

	return lastTitle, found
}
