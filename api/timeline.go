package api

import (
	"encoding/json"
	"github.com/buger/jsonparser"
	"github.com/GaPhi/gphotosuploader/auth"
)

// TimelineEntry represents an entry of the timeline
type TimelineEntry struct {
	// From
	from int64

	// To
	to int64

	// Media Items Count
	mediaCount int64
}

// Get whole timeline
func GetWholeTimeline(credentials auth.CookieCredentials) ([]TimelineEntry, error) {
	var (
		nextPageToken interface{}
		allEntries    = []TimelineEntry{}
		entries       []TimelineEntry
		err           error
	)

	// Fetch all pages, 100 entries at once
	for {
		entries, nextPageToken, err = GetTimelineEntries(credentials, nextPageToken)
		if err != nil {
			return allEntries, err
		}
		allEntries = append(allEntries, entries...)
		if nextPageToken == nil || nextPageToken == "" {
			// Return result
			return allEntries, nil
		}
	}
}

// Get timeline entries by page
func GetTimelineEntries(credentials auth.CookieCredentials, pageToken interface{}) ([]TimelineEntry, interface{}, error) {
	innerJson := []interface{}{
		pageToken, // Page token
		nil,
		1,
	}
	innerJsonString, err := json.Marshal(innerJson)
	if err != nil {
		return nil, nil, err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"rJ0tlb",
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

	entries := []TimelineEntry{}
	_, _ = jsonparser.ArrayEach(innerJsonRes, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		var entry TimelineEntry
		entry.from, err = jsonparser.GetInt(value, "[0]")
		if err != nil {
			return
		}
		entry.to, err = jsonparser.GetInt(value, "[1]")
		if err != nil {
			return
		}
		entry.mediaCount, err = jsonparser.GetInt(value, "[2]")
		if err != nil {
			return
		}
		entries = append(entries, entry)
	}, "[1]")

	nextPageToken, err := jsonparser.GetString(innerJsonRes, "[2]")
	if err != nil {
		return entries, nil, nil
	}
	return entries, nextPageToken, nil
}
