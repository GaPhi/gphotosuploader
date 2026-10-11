package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/GaPhi/gphotosuploader/api"
	"github.com/GaPhi/gphotosuploader/auth"
)

// Files which are already uploaded or not images/videos never reach Google
func TestConcurrentEnqueueOfIgnoredFiles(t *testing.T) {
	u, err := NewUploader(auth.CookieCredentials{}, "", 4)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	notMedia := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notMedia, []byte("not an image"), 0644); err != nil {
		t.Fatal(err)
	}
	uploaded := filepath.Join(dir, "IMG.JPG")

	const count = 50
	ignored := make(chan int)
	go func() {
		n := 0
		for range u.IgnoredUploads {
			n++
			if n == 2*count {
				ignored <- n
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			u.AddUploadedFiles(uploaded)
			_ = u.EnqueueUpload(uploaded)
		}()
		go func() {
			defer wg.Done()
			_ = u.EnqueueUpload(notMedia)
		}()
	}
	wg.Wait()
	u.WaitUploadsCompleted()

	if n := <-ignored; n != 2*count {
		t.Errorf("%v ignored uploads, expected %v", n, 2*count)
	}
}

func TestUploadsPausedOnNoSpaceLeft(t *testing.T) {
	u, _ := NewUploader(auth.CookieCredentials{}, "", 1)
	if until := u.recordUploadError(fmt.Errorf("failure: %w: 16/15", api.ErrNoSpaceLeft)); until.IsZero() {
		t.Fatal("uploads not paused after a no space left error")
	}
	if u.isPaused().IsZero() {
		t.Error("isPaused doesn't report the pause")
	}
}

func TestUploadsPausedAfterConsecutiveErrors(t *testing.T) {
	u, _ := NewUploader(auth.CookieCredentials{}, "", 1)
	someError := errors.New("network error")

	// A success resets the count
	for i := 0; i < maxConsecutiveErrors-1; i++ {
		if !u.recordUploadError(someError).IsZero() {
			t.Fatalf("uploads paused after %v errors", i+1)
		}
	}
	u.recordUploadSuccess("/photo.jpg")
	for i := 0; i < maxConsecutiveErrors-1; i++ {
		if !u.recordUploadError(someError).IsZero() {
			t.Fatalf("uploads paused after a success and %v errors", i+1)
		}
	}
	if !u.isPaused().IsZero() {
		t.Fatal("uploads paused too early")
	}

	if u.recordUploadError(someError).IsZero() || u.isPaused().IsZero() {
		t.Fatalf("uploads not paused after %v consecutive errors", maxConsecutiveErrors)
	}

	// The pause ends
	u.pausedUntil = time.Now().Add(-time.Second)
	if !u.isPaused().IsZero() {
		t.Error("uploads still paused after the end of the pause")
	}
}
