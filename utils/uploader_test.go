package utils

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

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
