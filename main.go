package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/GaPhi/gphotosuploader/api"
	"github.com/GaPhi/gphotosuploader/auth"
	"github.com/GaPhi/gphotosuploader/utils"
	"github.com/GaPhi/gphotosuploader/version"
	"github.com/fsnotify/fsnotify"
)

const noDeleteBefore = -1 << 63

var (
	// CLI arguments
	authFile             string
	queryStorage         bool
	deleteUnsupported    bool
	deleteBefore         int64
	emptyTrash           bool
	deleteEmptyAlbums    bool
	filesToUpload        utils.FilesToUpload
	directoriesToWatch   utils.DirectoriesToWatch
	albumId              string
	albumName            string
	albumSortKind        int
	shareWithUser        string
	sharedAlbumId        string
	uploadedListFile     string
	watchRecursively     bool
	maxConcurrentUploads int
	eventDelay           time.Duration
	printVersion         bool

	// Uploader
	uploader *utils.ConcurrentUploader
	timers   = make(map[string]*time.Timer)

	// Statistics
	uploadedFilesCount = 0
	ignoredCount       = 0
	errorsCount        = 0
)

func main() {
	parseCliArguments()
	if printVersion {
		fmt.Printf("Hash:\t%s\nCommit date:\t%s\n", version.Hash, version.Date)
		os.Exit(0)
	}

	credentials := initAuthentication()

	var err error

	// Get timeline
	if false {
		timeline, err := api.GetWholeTimeline(credentials)
		if err != nil {
			log.Fatalf("Can't get timeline: %v\n", err)
		}
		log.Printf("Got timeline: %v\n", len(timeline))
	}

	// Query storage
	if queryStorage {
		log.Printf("Querying storage...\n")
		used, total, err := api.QueryStorage(credentials)
		if err != nil {
			log.Fatalf("Can't get storage data: %v\n", err)
		}
		log.Printf("Storage: %v/%v (%v%%)\n", used, total, 100.0*used/total)
	}

	// Delete unsupported media items
	if deleteUnsupported {
		log.Printf("Deleting unsupported media items...\n")
		unsupported, err := api.ListAllUnsupportedMediaItemsBefore(credentials, nil)
		if err != nil {
			log.Fatalf("Can't get unsupported media items: %v\n", err)
		}
		log.Printf("Got unsupported media items: %v\n", len(unsupported))

		// No item?
		if len(unsupported) > 0 {
			// Get ids as an array
			ids := make([]string, len(unsupported))
			for i, mediaItem := range unsupported {
				ids[i] = mediaItem.MediaItemId
			}

			// Immediately delete (kind=2) too old media items
			err = api.DeleteMediaItems(credentials, ids, 2)
			if err != nil {
				log.Printf("Media items deletion FAILED: %v\n", err)
			} else {
				log.Printf("%v media items deleted\n", len(ids))
			}
		}
	}

	// Empty trash
	if emptyTrash {
		log.Printf("Empty trash...\n")
		err = api.EmptyTrash(credentials)
		if err != nil {
			log.Fatalf("Can't empty trash: %v\n", err)
		}
		log.Printf("Trash emptied\n")
	}

	// Delete too old media items
	if deleteBefore != noDeleteBefore {
		log.Printf("Deleting media items before %v...\n", time.Unix(0, deleteBefore*1000000).Local())
		mediaItems, err := api.ListAllMediaItemsBefore(credentials, deleteBefore, func(mediaItemsPart []api.MediaItem, err error) {
			// No item?
			if len(mediaItemsPart) == 0 {
				return
			}

			// Get ids as an array
			ids := make([]string, len(mediaItemsPart))
			for i, mediaItem := range mediaItemsPart {
				ids[i] = mediaItem.MediaItemId
			}

			// Immediately delete (kind=2) too old media items
			err = api.DeleteMediaItems(credentials, ids, 2)
			if err != nil {
				log.Printf("Media items deletion FAILED: %v\n", err)
			} else {
				log.Printf("%v media items deleted between %v and %v\n",
					len(ids),
					time.Unix(0, mediaItemsPart[len(mediaItemsPart)-1].StartDate*1000000).Local(),
					time.Unix(0, mediaItemsPart[0].StartDate*1000000).Local())
			}
		})
		if err != nil {
			log.Fatalf("Can't delete old media items: %v\n", err)
		}
		log.Printf("Media items deleted: %v\n", len(mediaItems))
	}

	// Delete empty albums
	if deleteEmptyAlbums {
		log.Printf("Deleting empty albums...\n")
		albums, deleted, notDeleted, err := api.DeleteEmptyAlbums(credentials)
		for _, album := range deleted {
			log.Printf("Empty album %v (%v) deleted\n", album.AlbumName, album.AlbumId)
		}
		for _, album := range notDeleted {
			log.Printf("Empty album %v (%v) deletion FAILED\n", album.AlbumName, album.AlbumId)
		}
		log.Printf("Album listed: %v\n", len(albums))
		if err != nil {
			log.Printf("Can't list albums: %v\n", err)
		}
	}

	// Create Album first to get albumId
	if albumName != "" {
		albumId, err = api.CreateAlbum(credentials, albumName)
		if err != nil {
			log.Fatalf("Can't create album: %v\n", err)
		}
		log.Printf("New album with ID '%v' created\n", albumId)
	}

	// Set album sort kind
	if albumSortKind != 0 {
		err = api.AlbumSortMediaItems(credentials, albumId, albumSortKind)
		if err != nil {
			log.Fatalf("Can't set album sort kind %v: %v\n", albumSortKind, err)
		}
		log.Printf("Album sort kind set to %v\n", albumSortKind)
	}

	// Share Album with a Google user
	if shareWithUser != "" {
		if len(albumId) == 44 {
			sharedAlbumId, err = api.AlbumShareWithUser(credentials, albumId, shareWithUser)
			if err != nil {
				log.Fatalf("Can't share album: %v\n", err)
			}
			log.Printf("Sharing album '%v' with user '%v' as '%v'\n", albumId, shareWithUser, sharedAlbumId)
		} else if len(albumId) == 70 {
			err = api.AlbumShareAddUser(credentials, albumId, shareWithUser)
			if err != nil {
				log.Fatalf("Can't add user to shared album: %v\n", err)
			}
			log.Printf("User '%v' added to shared album '%v'\n", shareWithUser, albumId)
		} else {
			log.Fatalf("Album Id length unrecognized: %v\n", albumId)
		}
	}

	uploader, err = utils.NewUploader(credentials, albumId, maxConcurrentUploads)
	if err != nil {
		log.Fatalf("Can't create uploader: %v\n", err)
	}

	stopHandler := make(chan bool)
	go handleUploaderEvents(stopHandler)

	loadAlreadyUploadedFiles()

	// Upload files passed as arguments
	uploadArgumentsFiles()

	// Wait until all the uploads are completed
	uploader.WaitUploadsCompleted()

	// Start to watch all the directories if needed
	if len(directoriesToWatch) > 0 {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			panic(err)
		}
		defer func(watcher *fsnotify.Watcher) {
			_ = watcher.Close()
		}(watcher)
		go handleFileSystemEvents(watcher, stopHandler)

		// Add all the directories passed as argument to the watcher
		for _, name := range directoriesToWatch {
			if err := startToWatch(name, watcher); err != nil {
				panic(err)
			}
		}

		log.Println("Watching 👀\nPress CTRL + C to stop")

		// Wait for CTRL + C
		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
	}

	stopHandler <- true
	<-stopHandler
	stopHandler <- true
	<-stopHandler

	log.Printf("Done (%v files uploaded, %v files ignored, %v errors)", uploadedFilesCount, ignoredCount, errorsCount)
	os.Exit(0)
}

// Parse CLI arguments
func parseCliArguments() {
	flag.StringVar(&authFile, "auth", "auth.json", "Authentication json file")
	flag.BoolVar(&queryStorage, "queryStorage", false, "Query storage (used/left)")
	flag.BoolVar(&deleteUnsupported, "deleteUnsupported", false, "Delete unsupported media items")
	flag.Int64Var(&deleteBefore, "deleteBefore", noDeleteBefore, "Use this parameter to delete existing media items created before this date (Unix timestamp in ms)")
	flag.BoolVar(&emptyTrash, "emptyTrash", false, "Empty trash")
	flag.BoolVar(&deleteEmptyAlbums, "deleteEmptyAlbums", false, "Delete empty albums")
	flag.Var(&filesToUpload, "upload", "File or directory to upload")
	flag.StringVar(&albumId, "album", "", "Use this parameter to move new images to a specific album")
	flag.StringVar(&albumName, "albumName", "", "Use this parameter to move new images to a new album")
	flag.IntVar(&albumSortKind, "albumSortKind", 0, "Use this parameter to set sort kind of the album (1: Newest first, 2: Oldest first, 3: Last added first)")
	flag.StringVar(&shareWithUser, "shareWithUser", "", "Use this parameter to share a specific album with a Google userId or userEmail")
	flag.StringVar(&uploadedListFile, "uploadedList", "uploaded.txt", "List to already uploaded files")
	flag.IntVar(&maxConcurrentUploads, "maxConcurrent", 1, "Number of max concurrent uploads")
	flag.Var(&directoriesToWatch, "watch", "Directory to watch")
	flag.BoolVar(&watchRecursively, "watchRecursively", true, "Start watching new directories in currently watched directories")
	delay := flag.Int("eventDelay", 3, "Distance of time to wait to consume different events of the same file (seconds)")
	flag.BoolVar(&api.LogRequests, "debug", false, "Log Google requests and responses")
	flag.BoolVar(&printVersion, "version", false, "Print version and commit date")

	flag.Parse()

	// Check flags
	if deleteBefore > time.Now().UnixNano()/1000000 {
		log.Fatalf("Invalid delteBefore date (after now)\n")
	}
	if albumId != "" && albumName != "" {
		log.Fatalf("Can't use album and albumName at the same time\n")
	}

	// Convert delay as int into duration
	eventDelay = time.Duration(*delay) * time.Second
}

func initAuthentication() auth.CookieCredentials {
	// Load authentication parameters
	credentials, err := auth.NewCookieCredentialsFromFile(authFile)
	if err != nil {
		log.Printf("Can't use '%v' as auth file\n", authFile)
		credentials = nil
	} else {
		log.Println("Auth file loaded, checking validity ...")
		validity, err := credentials.CheckCredentials()
		if err != nil {
			log.Fatalf("Can't check validity of credentials (%v)\n", err)
		} else if !validity.Valid {
			log.Printf("Credentials are not valid! %v\n", validity.Reason)
			credentials = nil
		} else {
			log.Println("Auth file seems to be valid")

			// Try to update auth file
			err = credentials.SerializeToFile(authFile)
			if err != nil {
				log.Printf("Can't update auth file %v: %v\n", authFile, err)
			}
		}
	}

	if credentials == nil {
		fmt.Println("The uploader can't continue without valid authentication tokens ...")
		fmt.Println("Would you like to run the WebDriver CookieCredentials Wizard ? [Yes/No]")
		fmt.Println("(If you don't know what it is, refer to the README)")

		var answer string
		_, _ = fmt.Scanln(&answer)
		startWizard := len(answer) > 0 && strings.ToLower(answer)[0] == 'y'

		if !startWizard {
			log.Fatalln("It's not possible to continue, sorry!")
		} else {
			credentials, err = utils.StartWebDriverCookieCredentialsWizard()
			if err != nil {
				log.Fatalf("Can't complete the login wizard, got: %v\n", err)
			} else {
				// Write auth file
				err = credentials.SerializeToFile(authFile)
				if err != nil {
					log.Fatalf("Can't write auth file %v: %v\n", authFile, err)
				}
			}
		}
	}

	// Get a new At token
	log.Println("Getting a new At token ...")
	token, err := api.NewAtTokenScraper(*credentials).ScrapeNewAtToken()
	if err != nil {
		log.Fatalf("Can't scrape a new At token (%v)\n", err)
	}
	credentials.RuntimeParameters.AtToken = token
	log.Println("At token taken")

	return *credentials
}

// Upload all the file and directories passed as arguments, calling filepath.Walk on each name
func uploadArgumentsFiles() {
	for _, name := range filesToUpload {
		_ = filepath.Walk(name, func(path string, file os.FileInfo, err error) error {
			if !file.IsDir() {
				_ = uploader.EnqueueUpload(path)
			}

			return nil
		})
	}
}

func handleUploaderEvents(exiting chan bool) {
	for {
		select {
		case info := <-uploader.CompletedUploads:
			uploadedFilesCount++
			log.Printf("Upload of '%v' completed\n", info)

			// Update the upload completed file
			if file, err := os.OpenFile(uploadedListFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666); err != nil {
				log.Println("Can't update the uploaded file list")
			} else {
				_, _ = file.WriteString(info + "\n")
				_ = file.Close()
			}

		case info := <-uploader.IgnoredUploads:
			ignoredCount++
			log.Printf("Not uploading '%v', it's already been uploaded or it's not a image/video!\n", info)

		case err := <-uploader.Errors:
			log.Printf("Upload error: %v\n", err)
			errorsCount++

		case <-exiting:
			exiting <- true
			break
		}
	}
}

func startToWatch(filePath string, fsWatcher *fsnotify.Watcher) error {
	if watchRecursively {
		return filepath.Walk(filePath, func(path string, file os.FileInfo, err error) error {
			if file.IsDir() {
				return fsWatcher.Add(path)
			}
			return nil
		})
	} else {
		return fsWatcher.Add(filePath)
	}
}

func handleFileChange(event fsnotify.Event, fsWatcher *fsnotify.Watcher) {
	// Use a map of timer to ignore different consecutive events for the same file.
	// (when the os writes a file to the disk, sometimes it repetitively sends same events)
	if timer, exists := timers[event.Name]; exists {

		// Cancel the timer
		cancelled := timer.Stop()

		if cancelled && event.Op != fsnotify.Remove && event.Op != fsnotify.Rename {
			// Postpone the file upload
			timer.Reset(eventDelay)
		}
	} else if event.Op != fsnotify.Remove && event.Op != fsnotify.Rename {
		timer = time.AfterFunc(eventDelay, func() {
			log.Printf("Finally consuming events for the %v file", event.Name)

			if info, err := os.Stat(event.Name); err != nil {
				log.Println(err)
			} else if !info.IsDir() {
				// Upload file
				_ = uploader.EnqueueUpload(event.Name)
			} else if watchRecursively {
				_ = startToWatch(event.Name, fsWatcher)
			}
		})
		timers[event.Name] = timer
	}
}

func handleFileSystemEvents(fsWatcher *fsnotify.Watcher, exiting chan bool) {
	for {
		select {
		case event := <-fsWatcher.Events:
			handleFileChange(event, fsWatcher)

		case err := <-fsWatcher.Errors:
			log.Println(err)

		case <-exiting:
			exiting <- true
			return
		}
	}
}

func loadAlreadyUploadedFiles() {
	file, err := os.OpenFile(uploadedListFile, os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		uploader.AddUploadedFiles(scanner.Text())
	}
}
