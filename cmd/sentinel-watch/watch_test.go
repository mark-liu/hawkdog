package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestReRegisterAfterDelete(t *testing.T) {
	// Shorten the re-register delay for testing.
	origDelay := reRegisterDelay
	reRegisterDelay = 100 * time.Millisecond
	t.Cleanup(func() { reRegisterDelay = origDelay })

	dir := t.TempDir()
	sentinel := filepath.Join(dir, "sentinel.ini")

	// Create the sentinel file.
	if err := ensureSentinel(sentinel); err != nil {
		t.Fatalf("ensureSentinel: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel not created: %v", err)
	}

	// Set up inotify.
	fd, err := unix.InotifyInit1(0)
	if err != nil {
		t.Fatalf("InotifyInit1: %v", err)
	}
	defer unix.Close(fd)

	wd, err := addWatch(fd, sentinel)
	if err != nil {
		t.Fatalf("addWatch: %v", err)
	}
	if wd < 0 {
		t.Fatal("expected valid watch descriptor")
	}

	// Delete the sentinel file — triggers IN_DELETE_SELF.
	if err := os.Remove(sentinel); err != nil {
		t.Fatalf("remove sentinel: %v", err)
	}

	// Read the deletion event from inotify.
	buf := make([]byte, 4096)
	n, err := unix.Read(fd, buf)
	if err != nil {
		t.Fatalf("read inotify: %v", err)
	}
	if n <= 0 {
		t.Fatal("expected inotify event data")
	}

	// Simulate the re-registration logic from watch().
	time.Sleep(reRegisterDelay)

	if err := ensureSentinel(sentinel); err != nil {
		t.Fatalf("re-create sentinel: %v", err)
	}

	// Verify sentinel was re-created.
	info, err := os.Stat(sentinel)
	if err != nil {
		t.Fatalf("sentinel not re-created: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("sentinel perms = %v, want 0600", info.Mode().Perm())
	}

	// Re-register watch on the new file.
	wd2, err := addWatch(fd, sentinel)
	if err != nil {
		t.Fatalf("re-register watch: %v", err)
	}
	if wd2 < 0 {
		t.Fatal("expected valid watch descriptor after re-registration")
	}

	// Verify the new watch fires on access.
	f, err := os.Open(sentinel)
	if err != nil {
		t.Fatalf("open sentinel: %v", err)
	}
	f.Close()

	n, err = unix.Read(fd, buf)
	if err != nil {
		t.Fatalf("read inotify after re-register: %v", err)
	}
	if n <= 0 {
		t.Fatal("expected inotify event after re-registration")
	}
}
