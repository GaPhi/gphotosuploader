package utils

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/GaPhi/gphotosuploader/api"
	"github.com/GaPhi/gphotosuploader/auth"
)

const (
	// Number of consecutive upload errors after which the uploads are paused
	maxConsecutiveErrors = 3

	// Duration of a pause of the uploads, after which watch mode tries again
	uploadsPauseDuration = time.Hour
)

// Simple client used to implement the tool that can upload multiple photos or videos at once
type ConcurrentUploader struct {
	credentials auth.CookieCredentials

	// Optional field to specify the destination album
	albumId string

	// Buffered channel to limit concurrent uploads
	concurrentLimiter chan bool

	// Protects uploadedFiles, waiting, consecutiveErrors and pausedUntil, used by concurrent uploads and file system events
	mutex sync.Mutex

	// Map of uploaded files (used as a set)
	uploadedFiles map[string]bool

	// Waiting group used for the implementation of the Wait method
	waitingGroup sync.WaitGroup

	// Flag to indicate if the client is waiting for all the upload to finish
	waiting bool

	// Number of upload errors since the last successful upload
	consecutiveErrors int

	// No upload shall be attempted before this time (after a "no space left" error or too many consecutive errors)
	pausedUntil time.Time

	CompletedUploads chan string
	IgnoredUploads   chan string
	Errors           chan error
}

// Creates a new ConcurrentUploader using the specified credentials.
// The second argument is the id of the album in which images are going to be added when uploaded. Use an empty string
// if you don't want to move the images in to a specific album. The third argument is the maximum number of concurrent
// uploads (which must not be 0).
func NewUploader(credentials auth.CookieCredentials, albumId string, maxConcurrentUploads int) (*ConcurrentUploader, error) {
	if maxConcurrentUploads <= 0 {
		return nil, fmt.Errorf("maxConcurrentUploads must be greater than zero")
	}

	return &ConcurrentUploader{
		credentials: credentials,
		albumId:     albumId,

		concurrentLimiter: make(chan bool, maxConcurrentUploads),

		uploadedFiles: make(map[string]bool),

		CompletedUploads: make(chan string),
		IgnoredUploads:   make(chan string),
		Errors:           make(chan error),
	}, nil
}

// Add files to the list of already uploaded files
func (u *ConcurrentUploader) AddUploadedFiles(files ...string) {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	for _, name := range files {
		u.uploadedFiles[name] = true
	}
}

// Enqueue a new upload. You must not call this method while waiting for some uploads to finish (The method return an
// error if you try to do it).
// Due to the fact that this method is asynchronous, if nil is return it doesn't mean the the upload was completed:
// for that use the Errors and CompletedUploads channels
func (u *ConcurrentUploader) EnqueueUpload(filePath string) error {
	// We need to use the absolute path of the file, to avoid multiple uploads of the same file if the tool is executed
	// from different directories
	if !filepath.IsAbs(filePath) {
		if abs, err := filepath.Abs(filePath); err != nil {
			log.Printf("uploader: Can't get the absolute path of file to upload, using relative path. Error: %v\n", err)
		} else {
			filePath = abs
		}
	}

	if u.wasFileAlreadyUploaded(filePath) {
		u.IgnoredUploads <- filePath
		return nil
	}

	// Check if the file is an image or a video
	if valid, err := IsImageOrVideo(filePath); err != nil {
		u.sendError(filePath, err)
		return nil
	} else if !valid {
		u.IgnoredUploads <- filePath
		return nil
	}

	// Join the group while holding the mutex, so that WaitUploadsCompleted can't miss this upload
	u.mutex.Lock()
	if u.waiting {
		u.mutex.Unlock()
		return fmt.Errorf("can't add new uploads while waiting queued uploads to finish")
	}
	if pausedUntil := u.pausedUntil; time.Now().Before(pausedUntil) {
		u.mutex.Unlock()
		u.sendError(filePath, fmt.Errorf("uploads paused until %v", pausedUntil.Local().Format(time.DateTime)))
		return nil
	}
	u.waitingGroup.Add(1)
	u.mutex.Unlock()

	go u.uploadFile(filePath)

	return nil
}

func (u *ConcurrentUploader) wasFileAlreadyUploaded(filePath string) bool {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	_, uploaded := u.uploadedFiles[filePath]
	return uploaded
}

// Returns the time until which the uploads are paused, zero if they aren't
func (u *ConcurrentUploader) isPaused() time.Time {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	if time.Now().Before(u.pausedUntil) {
		return u.pausedUntil
	}
	return time.Time{}
}

// Pause the uploads when the storage is full or after too many consecutive errors (expired cookies, quota, network...),
// instead of failing the same way for every file. Returns the end of the pause if the uploads are now paused.
func (u *ConcurrentUploader) recordUploadError(err error) time.Time {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	u.consecutiveErrors++
	if errors.Is(err, api.ErrNoSpaceLeft) || u.consecutiveErrors >= maxConsecutiveErrors {
		u.consecutiveErrors = 0
		u.pausedUntil = time.Now().Add(uploadsPauseDuration)
		return u.pausedUntil
	}
	return time.Time{}
}

func (u *ConcurrentUploader) recordUploadSuccess(filePath string) {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	u.consecutiveErrors = 0
	u.uploadedFiles[filePath] = true
}

func (u *ConcurrentUploader) uploadFile(filePath string) {
	u.waitForTurn()
	defer u.leaveGroupAndNotifyNextUpload()

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		u.sendError(filePath, err)
		return
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	// Create options
	options, err := api.NewUploadOptionsFromFile(file)
	if err != nil {
		u.sendError(filePath, err)
		return
	}
	options.AlbumId = u.albumId

	// Create a new upload
	upload, err := api.NewUpload(options, u.credentials)
	if err != nil {
		u.sendError(filePath, err)
		return
	}

	// Try to upload the image
	if pausedUntil := u.isPaused(); !pausedUntil.IsZero() {
		u.sendError(filePath, fmt.Errorf("uploads paused until %v", pausedUntil.Local().Format(time.DateTime)))
	} else if result, err := upload.Upload(); err != nil && (result == nil || result.ImageUrl == "") {
		if pausedUntil := u.recordUploadError(err); !pausedUntil.IsZero() {
			log.Printf("uploader: uploads paused until %v\n", pausedUntil.Local().Format(time.DateTime))
		}
		u.sendError(filePath, err)
	} else {
		// The media item is created even if its id couldn't be read (a warning is logged): don't upload it again
		u.recordUploadSuccess(filePath)
		u.CompletedUploads <- filePath
	}
}

func (u *ConcurrentUploader) sendError(filePath string, err error) {
	u.Errors <- fmt.Errorf("error with '%s': %s", filePath, err)
}

func (u *ConcurrentUploader) waitForTurn() {
	// Insert something in the channel. We remove values from it only when we complete an upload, blocking the
	// goroutines if we exceed the maxConcurrentUpload
	u.concurrentLimiter <- true
}

func (u *ConcurrentUploader) leaveGroupAndNotifyNextUpload() {
	// Remove a value to empty the channel or to unlock a waiting gorutine
	<-u.concurrentLimiter

	u.waitingGroup.Done()
}

// Blocks this goroutine until all the upload are completed. You can not add uploads when a goroutine call this method
func (u *ConcurrentUploader) WaitUploadsCompleted() {
	u.mutex.Lock()
	u.waiting = true
	u.mutex.Unlock()

	u.waitingGroup.Wait()

	u.mutex.Lock()
	u.waiting = false
	u.mutex.Unlock()
}
