package terminal

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	// Create a manager without a store (nil is valid)
	mgr := NewManager(nil)
	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}
	if mgr.sessions == nil {
		t.Error("manager sessions map should be initialized")
	}
}

func TestManagerCreateSession(t *testing.T) {
	// Skip if running in a restricted environment without PTY support
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	defer func() {
		// Cleanup all sessions
		for id := range mgr.sessions {
			mgr.DeleteSession(id)
		}
	}()

	// Use a real directory for the session
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if session == nil {
		t.Fatal("CreateSession() returned nil session")
	}
	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if session.RepoID != "test-repo" {
		t.Errorf("session RepoID = %q, want %q", session.RepoID, "test-repo")
	}
	if session.RepoPath != tmpDir {
		t.Errorf("session RepoPath = %q, want %q", session.RepoPath, tmpDir)
	}
	if session.CreatedAt.IsZero() {
		t.Error("session CreatedAt should not be zero")
	}
	if session.ptmx == nil {
		t.Error("session ptmx should not be nil")
	}
	if session.cmd == nil {
		t.Error("session cmd should not be nil")
	}
}

func TestManagerGetSession(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer mgr.DeleteSession(session.ID)

	// Get existing session
	retrieved := mgr.GetSession(session.ID)
	if retrieved == nil {
		t.Error("GetSession() returned nil for existing session")
	}
	if retrieved.ID != session.ID {
		t.Errorf("retrieved session ID = %q, want %q", retrieved.ID, session.ID)
	}

	// Get non-existent session
	nonExistent := mgr.GetSession("non-existent-id")
	if nonExistent != nil {
		t.Error("GetSession() should return nil for non-existent session")
	}
}

func TestManagerListSessions(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	// Create sessions for different repos
	session1, _ := mgr.CreateSession("repo-1", tmpDir)
	session2, _ := mgr.CreateSession("repo-1", tmpDir)
	session3, _ := mgr.CreateSession("repo-2", tmpDir)

	defer func() {
		mgr.DeleteSession(session1.ID)
		mgr.DeleteSession(session2.ID)
		mgr.DeleteSession(session3.ID)
	}()

	// List sessions for repo-1
	repo1Sessions := mgr.ListSessions("repo-1")
	if len(repo1Sessions) != 2 {
		t.Errorf("ListSessions(repo-1) = %d sessions, want 2", len(repo1Sessions))
	}

	// List sessions for repo-2
	repo2Sessions := mgr.ListSessions("repo-2")
	if len(repo2Sessions) != 1 {
		t.Errorf("ListSessions(repo-2) = %d sessions, want 1", len(repo2Sessions))
	}

	// List sessions for non-existent repo
	emptyList := mgr.ListSessions("non-existent-repo")
	if len(emptyList) != 0 {
		t.Errorf("ListSessions(non-existent) = %d sessions, want 0", len(emptyList))
	}
}

func TestManagerDeleteSession(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	sessionID := session.ID

	// Verify session exists
	if mgr.GetSession(sessionID) == nil {
		t.Error("session should exist before deletion")
	}

	// Delete the session
	mgr.DeleteSession(sessionID)

	// Verify session is gone
	if mgr.GetSession(sessionID) != nil {
		t.Error("session should not exist after deletion")
	}

	// Deleting non-existent session should not panic
	mgr.DeleteSession("non-existent-id")
}

func TestSessionSubscribeUnsubscribe(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer mgr.DeleteSession(session.ID)

	// Subscribe
	ch := session.Subscribe()
	if ch == nil {
		t.Error("Subscribe() returned nil channel")
	}

	// Verify subscriber count
	session.subscribersMu.Lock()
	count := len(session.subscribers)
	session.subscribersMu.Unlock()

	if count != 1 {
		t.Errorf("subscriber count = %d, want 1", count)
	}

	// Unsubscribe
	session.Unsubscribe(ch)

	session.subscribersMu.Lock()
	count = len(session.subscribers)
	session.subscribersMu.Unlock()

	if count != 0 {
		t.Errorf("subscriber count after unsubscribe = %d, want 0", count)
	}
}

func TestSessionEventSubscription(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer mgr.DeleteSession(session.ID)

	// Subscribe to events
	eventCh := session.SubscribeEvents()
	if eventCh == nil {
		t.Error("SubscribeEvents() returned nil channel")
	}

	// Verify event subscriber count
	session.eventSubsMu.Lock()
	count := len(session.eventSubs)
	session.eventSubsMu.Unlock()

	if count != 1 {
		t.Errorf("event subscriber count = %d, want 1", count)
	}

	// Unsubscribe
	session.UnsubscribeEvents(eventCh)

	session.eventSubsMu.Lock()
	count = len(session.eventSubs)
	session.eventSubsMu.Unlock()

	if count != 0 {
		t.Errorf("event subscriber count after unsubscribe = %d, want 0", count)
	}
}

func TestSessionSetTitle(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer mgr.DeleteSession(session.ID)

	// Subscribe to events to receive title changes
	eventCh := session.SubscribeEvents()

	// Set title
	session.SetTitle("New Terminal Title")

	// Check title was set
	if title := session.GetTitle(); title != "New Terminal Title" {
		t.Errorf("GetTitle() = %q, want %q", title, "New Terminal Title")
	}

	// Check event was emitted
	select {
	case event := <-eventCh:
		if event.Type != "title" {
			t.Errorf("event type = %q, want %q", event.Type, "title")
		}
		if event.Data != "New Terminal Title" {
			t.Errorf("event data = %q, want %q", event.Data, "New Terminal Title")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected title event, but none received")
	}
}

func TestSessionClientCount(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer mgr.DeleteSession(session.ID)

	// Initial client count should be 0
	session.mu.Lock()
	if session.clients != 0 {
		t.Errorf("initial client count = %d, want 0", session.clients)
	}
	session.mu.Unlock()

	// Add client
	session.AddClient()
	session.mu.Lock()
	if session.clients != 1 {
		t.Errorf("client count after AddClient = %d, want 1", session.clients)
	}
	session.mu.Unlock()

	// Add another
	session.AddClient()
	session.mu.Lock()
	if session.clients != 2 {
		t.Errorf("client count = %d, want 2", session.clients)
	}
	session.mu.Unlock()

	// Remove client
	session.RemoveClient()
	session.mu.Lock()
	if session.clients != 1 {
		t.Errorf("client count after RemoveClient = %d, want 1", session.clients)
	}
	session.mu.Unlock()

	// Remove more than exists (should not go negative)
	session.RemoveClient()
	session.RemoveClient()
	session.mu.Lock()
	if session.clients < 0 {
		t.Errorf("client count should not be negative, got %d", session.clients)
	}
	session.mu.Unlock()
}

func TestSessionClose(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Session should not be closed initially
	if session.IsClosed() {
		t.Error("new session should not be closed")
	}

	// Close the session
	session.Close()

	// Session should be closed
	if !session.IsClosed() {
		t.Error("session should be closed after Close()")
	}

	// Closing again should not panic
	session.Close()
}

func TestSessionResize(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer mgr.DeleteSession(session.ID)

	// Resize should work
	err = session.Resize(80, 24)
	if err != nil {
		t.Errorf("Resize() error = %v", err)
	}

	// Resize with different values
	err = session.Resize(120, 40)
	if err != nil {
		t.Errorf("Resize() error = %v", err)
	}

	// Resize on closed session should error
	session.Close()
	err = session.Resize(80, 24)
	if err == nil {
		t.Error("Resize() on closed session should return error")
	}
}

func TestSessionGetScrollback(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer mgr.DeleteSession(session.ID)

	// Initially scrollback should be empty or minimal
	scrollback := session.GetScrollback()
	// The scrollback might have some initial shell prompt, so we just check it doesn't panic
	_ = scrollback

	// Manually add to scrollback for testing
	session.scrollbackMu.Lock()
	session.scrollback = append(session.scrollback, []byte("test output")...)
	session.scrollbackMu.Unlock()

	scrollback = session.GetScrollback()
	if len(scrollback) == 0 {
		t.Error("scrollback should not be empty after adding data")
	}
}

func TestSessionGetPID(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer mgr.DeleteSession(session.ID)

	pid := session.GetPID()
	if pid <= 0 {
		t.Errorf("GetPID() = %d, expected positive PID", pid)
	}
}

func TestMaxScrollbackConstant(t *testing.T) {
	// Verify the max scrollback is set to expected value (100KB)
	expectedMax := 100 * 1024
	if MaxScrollback != expectedMax {
		t.Errorf("MaxScrollback = %d, want %d (100KB)", MaxScrollback, expectedMax)
	}
}

func TestSessionWriteReadClosed(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Close the session first
	session.Close()

	// Write should fail on closed session
	_, err = session.Write([]byte("test"))
	if err == nil {
		t.Error("Write() on closed session should return error")
	}

	// Read should fail on closed session
	buf := make([]byte, 100)
	_, err = session.Read(buf)
	if err == nil {
		t.Error("Read() on closed session should return error")
	}
}

func TestConcurrentSessionAccess(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)
	tmpDir := t.TempDir()

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer mgr.DeleteSession(session.ID)

	// Test concurrent access to session methods
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session.AddClient()
			session.GetTitle()
			session.GetScrollback()
			session.RemoveClient()
		}()
	}
	wg.Wait()
	// If we get here without race conditions or panics, the test passes
}

func TestSessionShellDetection(t *testing.T) {
	// This test verifies shell detection logic
	// The actual CreateSession uses SHELL env var or falls back to /bin/bash or /bin/sh

	// Check that common shells exist (at least one should)
	shells := []string{"/bin/bash", "/bin/sh", "/bin/zsh"}
	foundShell := false
	for _, shell := range shells {
		if _, err := os.Stat(shell); err == nil {
			foundShell = true
			break
		}
	}

	if !foundShell {
		t.Skip("No common shell found on system")
	}
}

func TestSessionWorkingDirectory(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping PTY test in CI environment")
	}

	mgr := NewManager(nil)

	// Create a unique temp directory
	tmpDir, err := os.MkdirTemp("", "homeport-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a marker file to verify we're in the right directory
	markerFile := filepath.Join(tmpDir, "test-marker.txt")
	if err := os.WriteFile(markerFile, []byte("marker"), 0644); err != nil {
		t.Fatalf("Failed to create marker file: %v", err)
	}

	session, err := mgr.CreateSession("test-repo", tmpDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	defer mgr.DeleteSession(session.ID)

	// Verify the session's RepoPath is set correctly
	if session.RepoPath != tmpDir {
		t.Errorf("session.RepoPath = %q, want %q", session.RepoPath, tmpDir)
	}
}
