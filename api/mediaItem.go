package api

import (
	"encoding/json"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/GaPhi/gphotosuploader/auth"
)

// MediaItem represents a media item (picture, video)
type MediaItem struct {
	// Media item identifier
	MediaItemId string

	// Media item content URL
	ContentUrl string

	// Media item width
	ContentWidth int64

	// Media item height
	ContentHeight int64

	// Date of the media item start
	StartDate int64

	// Date of the media item end
	EndDate int64

	// Some kind of media item serial number?
	MediaItemSn int64

	// Media item download URL
	DownloadUrl string

	// Filename of the media item
	Filename string
}

// List all media items before a date
func ListAllMediaItemsBefore(credentials auth.CookieCredentials, before interface{}, cb func([]MediaItem, error)) ([]MediaItem, error) {
	var (
		nextPageToken interface{}
		allMediaItems = []MediaItem{}
		mediaItems    []MediaItem
		err           error
	)

	// Fetch all pages, several media items at once
	for {
		mediaItems, nextPageToken, err = ListMediaItems(credentials, before, nextPageToken)
		if cb != nil {
			cb(mediaItems, err)
		}
		if err != nil {
			return allMediaItems, err
		}
		allMediaItems = append(allMediaItems, mediaItems...)
		if nextPageToken == nil || nextPageToken == "" {
			// Return result
			return allMediaItems, nil
		}
	}
}

// List media items by page
func ListMediaItems(credentials auth.CookieCredentials, before interface{}, pageToken interface{}) ([]MediaItem, interface{}, error) {
	innerJson := []interface{}{
		pageToken, // Page token
		before,    // Before this date (in ms)
		nil,
		nil,
		true,
		1,
		nil, // Date?
		nil, // string(last fetched start date)
	}
	innerJsonString, err := json.Marshal(innerJson)
	if err != nil {
		return nil, nil, err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"lcxiM",
				string(innerJsonString),
				nil,
				"generic",
			},
		},
	}
	innerJsonRes, err := doRequest(credentials, jsonReq)
	if err != nil {
		return nil, nil, err
	}

	mediaItems := []MediaItem{}
	_, _ = jsonparser.ArrayEach(innerJsonRes, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		if err != nil {
			return
		}
		var mediaItem MediaItem
		mediaItem.MediaItemId, err = jsonparser.GetString(value, "[0]")
		if err != nil {
			return
		}
		mediaItem.ContentUrl, err = jsonparser.GetString(value, "[1]", "[0]")
		if err != nil {
			return
		}
		mediaItem.ContentWidth, err = jsonparser.GetInt(value, "[1]", "[1]")
		if err != nil {
			return
		}
		mediaItem.ContentHeight, err = jsonparser.GetInt(value, "[1]", "[2]")
		if err != nil {
			return
		}
		mediaItem.StartDate, err = jsonparser.GetInt(value, "[2]")
		if err != nil {
			return
		}
		mediaItem.EndDate, err = jsonparser.GetInt(value, "[5]")
		if err != nil {
			return
		}
		// This data is not always present (2025-10-25: last array index is 9)
		// As this is not used by this tool, we just ignore the potential error
		mediaItem.MediaItemSn, _ = jsonparser.GetInt(value, "[14]")
		mediaItems = append(mediaItems, mediaItem)
	}, "[0]")

	nextPageToken, err := jsonparser.GetString(innerJsonRes, "[1]")
	if err != nil {
		return mediaItems, nil, nil
	}
	return mediaItems, nextPageToken, nil
}

// List all unsupported media items
func ListAllUnsupportedMediaItemsBefore(credentials auth.CookieCredentials, cb func([]MediaItem, error)) ([]MediaItem, error) {
	var (
		nextPageToken interface{}
		allMediaItems = []MediaItem{}
		mediaItems    []MediaItem
		err           error
	)

	// Fetch all pages, several media items at once
	for {
		mediaItems, nextPageToken, err = ListUnsupportedMediaItems(credentials, nextPageToken)
		if cb != nil {
			cb(mediaItems, err)
		}
		if err != nil {
			return allMediaItems, err
		}
		allMediaItems = append(allMediaItems, mediaItems...)
		if nextPageToken == nil || nextPageToken == "" {
			// Return result
			return allMediaItems, nil
		}
	}
}

// List unsupported media items by page
func ListUnsupportedMediaItems(credentials auth.CookieCredentials, pageToken interface{}) ([]MediaItem, interface{}, error) {
	innerJson := []interface{}{
		pageToken, // Page token
	}
	innerJsonString, err := json.Marshal(innerJson)
	if err != nil {
		return nil, nil, err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"TLvKMb",
				string(innerJsonString),
				nil,
				"generic",
			},
		},
	}
	innerJsonRes, err := doRequest(credentials, jsonReq)
	if err != nil {
		return nil, nil, err
	}

	mediaItems := []MediaItem{}
	_, _ = jsonparser.ArrayEach(innerJsonRes, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		if err != nil {
			return
		}
		var mediaItem MediaItem
		mediaItem.MediaItemId, err = jsonparser.GetString(value, "[0]")
		if err != nil {
			return
		}
		mediaItem.Filename, err = jsonparser.GetString(value, "[1]")
		if err != nil {
			return
		}
		mediaItem.MediaItemSn, err = jsonparser.GetInt(value, "[2]")
		if err != nil {
			return
		}
		mediaItem.StartDate, err = jsonparser.GetInt(value, "[3]")
		if err != nil {
			return
		}
		mediaItem.EndDate, err = jsonparser.GetInt(value, "[3]")
		if err != nil {
			return
		}
		mediaItem.ContentUrl, err = jsonparser.GetString(value, "[4]")
		if err != nil {
			return
		}
		mediaItem.DownloadUrl, err = jsonparser.GetString(value, "[5]")
		if err != nil {
			return
		}
		/*		mediaItem.XXXX, err = jsonparser.GetString(value, "[6]") // TODO: Identify this string
				if err != nil {
					return
				}*/
		mediaItems = append(mediaItems, mediaItem)
	}, "[1]")

	nextPageToken, err := jsonparser.GetString(innerJsonRes, "[0]")
	if err != nil {
		return mediaItems, nil, nil
	}
	return mediaItems, nextPageToken, nil
}

// DeleteMediaItems deletes or restores media items.
type DeleteKind int

const (
	SendToTrash DeleteKind = 1
	DeleteNow   DeleteKind = 2
	Restore     DeleteKind = 3
)

func DeleteMediaItems(credentials auth.CookieCredentials, mediaItemIds []string, kind DeleteKind) error {
	if len(mediaItemIds) == 0 {
		return nil
	}

	var actionCode int
	switch kind {
	case SendToTrash:
		actionCode = 3
	case DeleteNow, Restore:
		actionCode = 2
	default:
		return fmt.Errorf("invalid kind: %d", kind)
	}

	for i := 0; i < len(mediaItemIds); i += 250 {
		end := i + 250
		if end > len(mediaItemIds) {
			end = len(mediaItemIds)
		}
		ids := mediaItemIds[i:end]

		innerJson := []interface{}{
			nil,
			kind,
			ids,
			actionCode,
		}

		innerJsonString, err := json.Marshal(innerJson)
		if err != nil {
			return err
		}

		jsonReq := []interface{}{
			[]interface{}{
				[]interface{}{
					"XwAOJf",
					string(innerJsonString),
					nil,
					"generic",
				},
			},
		}
		_, err = doRequest(credentials, jsonReq)
		if err != nil {
			return err
		}
	}

	return nil
}
