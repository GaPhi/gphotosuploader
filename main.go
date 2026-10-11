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
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/GaPhi/gphotosuploader/api"
	"github.com/GaPhi/gphotosuploader/auth"
	"github.com/GaPhi/gphotosuploader/utils"
	"github.com/GaPhi/gphotosuploader/version"
	"github.com/fsnotify/fsnotify"
)

const noDeleteBefore = -1 << 63

// Number of media items sent to trash by each request
const deletePageSize = 250

// Delay between two cookies rotations during a run
const cookiesRotationInterval = 24 * time.Hour

// Minimum delay between two calls to Google to rotate the cookies, across runs (Google answers HTTP 429 when
// called more than about once a minute, and the cookies stay valid for weeks)
const cookiesRotationMinInterval = time.Hour

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

	// Serializes the cookies rotations and the writes of the auth file
	authFileMutex sync.Mutex

	// CTRL + C stops the watching instead of exiting
	watching     atomic.Bool
	stopWatching = make(chan struct{})

	// Uploader
	uploader *utils.ConcurrentUploader

	// Pending file system events, one timer per file (protected by timersMutex)
	timersMutex   sync.Mutex
	timers        = make(map[string]*time.Timer)
	timersStopped bool
	pendingEvents sync.WaitGroup

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

	// Keep the session alive during the whole run
	go func() {
		for range time.Tick(cookiesRotationInterval) {
			rotateAndSaveCookies(&credentials)
		}
	}()

	// Save the cookies before exiting on CTRL + C
	interrupted := make(chan os.Signal, 2)
	signal.Notify(interrupted, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-interrupted
		if watching.Load() {
			close(stopWatching)
			log.Println("Stopping, waiting for the running uploads (CTRL + C again to exit now)")
			<-interrupted
		}
		saveCookies(&credentials)
		log.Println("Interrupted")
		os.Exit(1)
	}()

	var err error

	// Get timeline
	if false {
		timeline, err := api.GetWholeTimeline(credentials)
		if err != nil {
			fatalf(&credentials, "Can't get timeline: %v\n", err)
		}
		log.Printf("Got timeline: %v\n", len(timeline))
	}

	// Delete unsupported media items
	if deleteUnsupported {
		log.Printf("Deleting unsupported media items...\n")
		unsupported, err := api.ListAllUnsupportedMediaItemsBefore(credentials, nil)
		if err != nil {
			fatalf(&credentials, "Can't get unsupported media items: %v\n", err)
		}
		log.Printf("Got unsupported media items: %v\n", len(unsupported))

		// No item?
		if len(unsupported) > 0 {
			// Get ids as an array
			ids := make([]string, len(unsupported))
			for i, mediaItem := range unsupported {
				ids[i] = mediaItem.MediaItemId
			}

			// Send too old media items to trash
			err = api.DeleteMediaItems(credentials, ids, api.SendToTrash)
			if err != nil {
				log.Printf("Media items deletion FAILED: %v\n", err)
			} else {
				log.Printf("%v media items deleted\n", len(ids))
			}
		}
	}

	// Delete too old media items
	if deleteBefore != noDeleteBefore {
		log.Printf("Deleting media items before %v...\n", time.Unix(0, deleteBefore*1000000).Local())
		// List everything first: deleting while paginating could make the next page skip media items
		mediaItems, err := api.ListAllMediaItemsBefore(credentials, deleteBefore, nil)
		if err != nil {
			fatalf(&credentials, "Can't list old media items: %v\n", err)
		}
		log.Printf("Got media items to delete: %v\n", len(mediaItems))

		// Send too old media items to trash, page by page
		deleted := 0
		for i := 0; i < len(mediaItems); i += deletePageSize {
			mediaItemsPart := mediaItems[i:min(i+deletePageSize, len(mediaItems))]
			ids := make([]string, len(mediaItemsPart))
			for i, mediaItem := range mediaItemsPart {
				ids[i] = mediaItem.MediaItemId
			}
			if err := api.DeleteMediaItems(credentials, ids, api.SendToTrash); err != nil {
				log.Printf("Media items deletion FAILED: %v\n", err)
				continue
			}
			deleted += len(ids)
			log.Printf("%v media items deleted between %v and %v\n",
				len(ids),
				time.Unix(0, mediaItemsPart[len(mediaItemsPart)-1].StartDate*1000000).Local(),
				time.Unix(0, mediaItemsPart[0].StartDate*1000000).Local())
		}
		log.Printf("Media items deleted: %v/%v\n", deleted, len(mediaItems))
	}

	// Empty trash
	if emptyTrash {
		log.Printf("Empty trash...\n")
		err = api.EmptyTrash(credentials)
		if err != nil {
			fatalf(&credentials, "Can't empty trash: %v\n", err)
		}
		log.Printf("Trash emptied\n")
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
			fatalf(&credentials, "Can't create album: %v\n", err)
		}
		log.Printf("New album with ID '%v' created\n", albumId)
	}

	// Set album sort kind
	if albumSortKind != 0 {
		err = api.AlbumSortMediaItems(credentials, albumId, albumSortKind)
		if err != nil {
			fatalf(&credentials, "Can't set album sort kind %v: %v\n", albumSortKind, err)
		}
		log.Printf("Album sort kind set to %v\n", albumSortKind)
	}

	// Share Album with a Google user
	if shareWithUser != "" {
		if len(albumId) == 44 {
			sharedAlbumId, err = api.AlbumShareWithUser(credentials, albumId, shareWithUser)
			if err != nil {
				fatalf(&credentials, "Can't share album: %v\n", err)
			}
			log.Printf("Sharing album '%v' with user '%v' as '%v'\n", albumId, shareWithUser, sharedAlbumId)
		} else if len(albumId) == 70 {
			err = api.AlbumShareAddUser(credentials, albumId, shareWithUser)
			if err != nil {
				fatalf(&credentials, "Can't add user to shared album: %v\n", err)
			}
			log.Printf("User '%v' added to shared album '%v'\n", shareWithUser, albumId)
		} else {
			fatalf(&credentials, "Album Id length unrecognized: %v\n", albumId)
		}
	}

	uploader, err = utils.NewUploader(credentials, albumId, maxConcurrentUploads)
	if err != nil {
		fatalf(&credentials, "Can't create uploader: %v\n", err)
	}

	stopHandler := make(chan struct{})
	handlerStopped := make(chan struct{})
	go handleUploaderEvents(stopHandler, handlerStopped)

	loadAlreadyUploadedFiles()

	// Upload files passed as arguments
	uploadArgumentsFiles()

	// Wait until all the uploads are completed
	uploader.WaitUploadsCompleted()

	// Start to watch all the directories if needed
	if len(directoriesToWatch) > 0 {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			fatalf(&credentials, "Can't create the file system watcher: %v\n", err)
		}
		go handleFileSystemEvents(watcher)

		// Add all the directories passed as argument to the watcher
		for _, name := range directoriesToWatch {
			if err := startToWatch(name, watcher); err != nil {
				fatalf(&credentials, "Can't watch %v: %v\n", name, err)
			}
		}

		log.Println("Watching 👀\nPress CTRL + C to stop")

		// Wait for CTRL + C
		watching.Store(true)
		<-stopWatching

		// Stop receiving events, then let the running uploads complete
		_ = watcher.Close()
		stopPendingEvents()
		pendingEvents.Wait()
		uploader.WaitUploadsCompleted()
	}

	close(stopHandler)
	<-handlerStopped

	// Query storage
	if queryStorage {
		log.Printf("Querying storage...\n")
		used, total, err := api.QueryStorage(credentials)
		if err != nil {
			fatalf(&credentials, "Can't get storage data: %v\n", err)
		}
		if total > 0 {
			log.Printf("Storage: %v/%v (%v%%)\n", used, total, 100*used/total)
		} else {
			log.Printf("Storage: %v used\n", used)
		}
	}

	// Keep the cookies updated by Google during the run
	saveCookies(&credentials)

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

			// Renew the short-lived cookies and update auth file
			rotateAndSaveCookies(credentials)
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
		fatalf(credentials, "Can't scrape a new At token (%v)\n", err)
	}
	credentials.RuntimeParameters.AtToken = token
	log.Println("At token taken")

	return *credentials
}

// Renew the short-lived session cookies, then write the cookies sent back by Google into the auth file.
// A failed rotation is not fatal: the current cookies are still valid at this point.
func rotateAndSaveCookies(credentials *auth.CookieCredentials) {
	authFileMutex.Lock()
	defer authFileMutex.Unlock()

	// Never rotate if the result can't be saved: the values kept in the auth file would be dropped by Google
	if err := checkAuthFileWritable(); err != nil {
		log.Printf("Auth file %v is not writable, cookies not rotated: %v\n", authFile, err)
		return
	}

	// Don't call Google again if the cookies were rotated recently, by this run or a previous one
	if credentials.RotatedWithin(cookiesRotationMinInterval) {
		log.Printf("Cookies rotated at %v, next rotation after %v\n",
			credentials.LastRotation().Local().Format(time.DateTime),
			credentials.LastRotation().Add(cookiesRotationMinInterval).Local().Format(time.DateTime))
		writeAuthFile(credentials)
		return
	}

	if rotated, err := credentials.RotateCookies(); err != nil {
		log.Printf("Can't rotate cookies: %v\n", err)
	} else if rotated {
		log.Println("Cookies rotated")
	} else {
		log.Println("Cookies not rotated (Google didn't send new ones)")
	}

	writeAuthFile(credentials)
}

// Write into the auth file the cookies currently held, including the ones updated by Google in any response
func saveCookies(credentials *auth.CookieCredentials) {
	authFileMutex.Lock()
	defer authFileMutex.Unlock()
	writeAuthFile(credentials)
}

// Same as saveCookies, authFileMutex being already locked
func writeAuthFile(credentials *auth.CookieCredentials) {
	if err := checkAuthFileWritable(); err != nil {
		log.Printf("Auth file %v is not writable, not updated: %v\n", authFile, err)
		return
	}
	if err := credentials.SerializeToFile(authFile); err != nil {
		log.Printf("Can't update auth file %v: %v\n", authFile, err)
	}
}

func checkAuthFileWritable() error {
	file, err := os.OpenFile(authFile, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	return file.Close()
}

// Save the cookies, then exit as log.Fatalf
func fatalf(credentials *auth.CookieCredentials, format string, v ...interface{}) {
	saveCookies(credentials)
	log.Fatalf(format, v...)
}

// Upload all the file and directories passed as arguments, calling filepath.Walk on each name
func uploadArgumentsFiles() {
	for _, name := range filesToUpload {
		_ = filepath.Walk(name, func(path string, file os.FileInfo, err error) error {
			if err != nil {
				log.Println(err)
				return nil
			}
			if utils.IsIgnoredPath(path) {
				if file.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !file.IsDir() {
				_ = uploader.EnqueueUpload(path)
			}

			return nil
		})
	}
}

func handleUploaderEvents(stop <-chan struct{}, stopped chan<- struct{}) {
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

		case <-stop:
			close(stopped)
			return
		}
	}
}

func startToWatch(filePath string, fsWatcher *fsnotify.Watcher) error {
	if watchRecursively {
		return filepath.Walk(filePath, func(path string, file os.FileInfo, err error) error {
			if err != nil {
				log.Println(err)
				return nil
			}
			if file.IsDir() {
				if utils.IsIgnoredPath(path) {
					return filepath.SkipDir
				}
				return fsWatcher.Add(path)
			}
			return nil
		})
	} else {
		return fsWatcher.Add(filePath)
	}
}

func handleFileChange(event fsnotify.Event, fsWatcher *fsnotify.Watcher) {
	if utils.IsIgnoredPath(event.Name) {
		return
	}

	// Use a map of timer to ignore different consecutive events for the same file.
	// (when the os writes a file to the disk, sometimes it repetitively sends same events)
	timersMutex.Lock()
	defer timersMutex.Unlock()
	if timersStopped {
		return
	}

	// Cancel the pending timer, if any
	if timer, exists := timers[event.Name]; exists {
		delete(timers, event.Name)
		if timer.Stop() {
			pendingEvents.Done()
		}
	}
	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		return
	}

	// (Re)start the delay before consuming the file
	var timer *time.Timer
	pendingEvents.Add(1)
	timer = time.AfterFunc(eventDelay, func() {
		defer pendingEvents.Done()

		timersMutex.Lock()
		if timers[event.Name] == timer {
			delete(timers, event.Name)
		}
		timersMutex.Unlock()

		log.Printf("Finally consuming events for the %v file", event.Name)

		if info, err := os.Stat(event.Name); err != nil {
			log.Println(err)
		} else if !info.IsDir() {
			// Upload file
			if err := uploader.EnqueueUpload(event.Name); err != nil {
				log.Printf("Not uploading '%v': %v\n", event.Name, err)
			}
		} else if watchRecursively {
			if err := startToWatch(event.Name, fsWatcher); err != nil {
				log.Printf("Can't watch %v: %v\n", event.Name, err)
			}
		}
	})
	timers[event.Name] = timer
}

// Cancel the file events not consumed yet and ignore the next ones
func stopPendingEvents() {
	timersMutex.Lock()
	defer timersMutex.Unlock()
	timersStopped = true
	for name, timer := range timers {
		if timer.Stop() {
			pendingEvents.Done()
			log.Printf("Not uploading '%v': stopped before the end of its event delay\n", name)
		}
		delete(timers, name)
	}
}

func handleFileSystemEvents(fsWatcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-fsWatcher.Events:
			if !ok {
				return
			}
			handleFileChange(event, fsWatcher)

		case err, ok := <-fsWatcher.Errors:
			if !ok {
				return
			}
			log.Println(err)
		}
	}
}

func loadAlreadyUploadedFiles() {
	file, err := os.OpenFile(uploadedListFile, os.O_CREATE|os.O_RDONLY, 0666)
	if err != nil {
		log.Fatalf("Can't open the uploaded files list %v: %v\n", uploadedListFile, err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		uploader.AddUploadedFiles(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Can't read the uploaded files list %v: %v\n", uploadedListFile, err)
	}
}
