package api

import (
	"encoding/json"
	"github.com/buger/jsonparser"
	"github.com/simonedegiacomi/gphotosuploader/auth"
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
}

// List all media items before a date
func ListAllMediaItemsBefore(credentials auth.CookieCredentials, before interface{}, cb func([]MediaItem, error)) ([]MediaItem, error) {
	var (
		nextPageToken interface{}
		allMediaItems []MediaItem = []MediaItem{}
		mediaItems    []MediaItem
		err           error
	)

	// Fetch all pages, several media items at once
	for {
		mediaItems, nextPageToken, err = ListMediaItems(credentials, before, nextPageToken)
		if err != nil {
			return allMediaItems, err
		}
		if cb != nil {
			cb(mediaItems, err)
		}
		if err != nil {
			return allMediaItems, err
		}
		allMediaItems = append(allMediaItems, mediaItems...)
		if nextPageToken == nil {
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

	var mediaItems []MediaItem = []MediaItem{}
	if len(innerJsonRes) > 2 {
		_, err = jsonparser.ArrayEach(innerJsonRes, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
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
			mediaItem.MediaItemSn, err = jsonparser.GetInt(value, "[14]")
			if err != nil {
				return
			}
			mediaItems = append(mediaItems, mediaItem)
		}, "[0]")
		if err != nil {
			return mediaItems, nil, unexpectedResponse(innerJsonRes)
		}
	}

	nextPageToken, err := jsonparser.GetString(innerJsonRes, "[1]")
	if err != nil {
		return mediaItems, nil, nil
	}
	return mediaItems, nextPageToken, nil
}

// DeleteMediaItems a media item
// kind=1 for Send to trash
// kind=2 for Immediate deletion
// kind=3 for Restore from trash
func DeleteMediaItems(credentials auth.CookieCredentials, mediaItemIds []string, kind int) error {
	innerJson := []interface{}{
		mediaItemIds,
		kind,
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
				nil,
				nil,
				"generic",
			},
		},
	}
	_, err = doRequest(credentials, jsonReq)
	if err != nil {
		return err
	}

	return nil
}
