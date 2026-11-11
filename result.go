package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

// Summary of a run, printed as a single JSON object on stdout with --json (the logs stay on stderr)
type runResult struct {
	ExitCode         int            `json:"exitCode"`
	Error            string         `json:"error,omitempty"`
	UserId           string         `json:"userId,omitempty"`
	CredentialsValid *bool          `json:"credentialsValid,omitempty"`
	Storage          *storageResult `json:"storage,omitempty"`
	AlbumId          string         `json:"albumId,omitempty"`
	SharedAlbumId    string         `json:"sharedAlbumId,omitempty"`
	DeletedItems     int            `json:"deletedItems"`
	DeletedAlbums    int            `json:"deletedAlbums"`
	Uploaded         []string       `json:"uploaded"`
	Ignored          int            `json:"ignored"`
	Errors           []string       `json:"errors"`
	NoSpaceLeft      bool           `json:"noSpaceLeft"`
}

type storageResult struct {
	Used  int64 `json:"used"`
	Total int64 `json:"total"`
}

var (
	jsonOutput  bool
	result      = runResult{Uploaded: []string{}, Errors: []string{}}
	resultMutex sync.Mutex
)

// Update the result under its mutex (uploads are reported by another goroutine)
func updateResult(update func(r *runResult)) {
	resultMutex.Lock()
	defer resultMutex.Unlock()
	update(&result)
}

// Exit with the given code, printing the result first with --json
func exit(code int) {
	if jsonOutput {
		resultMutex.Lock()
		result.ExitCode = code
		if err := json.NewEncoder(os.Stdout).Encode(&result); err != nil {
			log.Printf("Can't write the JSON result: %v\n", err)
		}
		resultMutex.Unlock()
	}
	os.Exit(code)
}

// Log the error, keep it in the result, then exit with the given code
func exitf(code int, format string, v ...interface{}) {
	log.Printf(format, v...)
	if jsonOutput {
		updateResult(func(r *runResult) { r.Error = strings.TrimRight(fmt.Sprintf(format, v...), "\n") })
	}
	exit(code)
}
