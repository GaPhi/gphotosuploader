package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GaPhi/gphotosuploader/auth"
	"github.com/GaPhi/gphotosuploader/utils"
	"github.com/fsnotify/fsnotify"
)

// Consecutive events of a file are consumed once, and a later change of the same file is consumed again
func TestFileEventsAreDebounced(t *testing.T) {
	var err error
	uploader, err = utils.NewUploader(auth.CookieCredentials{}, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	eventDelay = 50 * time.Millisecond
	timersStopped = false

	file := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(file, []byte("not an image"), 0644); err != nil {
		t.Fatal(err)
	}

	consumed := 0
	expectConsumed := func(expected int) {
		t.Helper()
		done := make(chan struct{})
		go func() {
			pendingEvents.Wait()
			close(done)
		}()
		for {
			select {
			case <-uploader.IgnoredUploads:
				consumed++
			case <-done:
				if consumed != expected {
					t.Fatalf("file consumed %v times, expected %v", consumed, expected)
				}
				return
			}
		}
	}

	for i := 0; i < 5; i++ {
		handleFileChange(fsnotify.Event{Name: file, Op: fsnotify.Write}, nil)
	}
	expectConsumed(1)

	handleFileChange(fsnotify.Event{Name: file, Op: fsnotify.Write}, nil)
	expectConsumed(2)

	// A removed file is not consumed
	handleFileChange(fsnotify.Event{Name: file, Op: fsnotify.Create}, nil)
	handleFileChange(fsnotify.Event{Name: file, Op: fsnotify.Remove}, nil)
	expectConsumed(2)

	// Pending events are dropped when stopping
	handleFileChange(fsnotify.Event{Name: file, Op: fsnotify.Write}, nil)
	stopPendingEvents()
	handleFileChange(fsnotify.Event{Name: file, Op: fsnotify.Write}, nil)
	expectConsumed(2)
}
