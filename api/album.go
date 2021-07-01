package api

import (
	"encoding/json"
	"github.com/buger/jsonparser"
	"github.com/simonedegiacomi/gphotosuploader/auth"
)

// Album represents an album
type Album struct {
	// Album identifier
	AlbumId string

	// Album name
	AlbumName string

	// Number of media items in the album
	MediaCount int64
}

// Create Album
func CreateAlbum(credentials auth.CookieCredentials, albumName string) (string, error) {
	innerJson := []interface{}{
		albumName,
		nil,
		2,
		[]interface{}{},
	}
	innerJsonString, err := json.Marshal(innerJson)
	if err != nil {
		return "", err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"OXvT9d",
				string(innerJsonString),
				nil,
				"generic",
			},
		},
	}
	innerJsonRes, err := doRequest(credentials, jsonReq)
	if err != nil {
		return "", err
	}

	albumId, err := jsonparser.GetString(innerJsonRes, "[0]", "[0]")
	if err != nil {
		return "", unexpectedResponse()
	}

	return albumId, nil
}

// Share Album
func AlbumShareWithUserId(credentials auth.CookieCredentials, albumId string, shareWithUserId string) (string, error) {
	innerJson := []interface{}{
		nil,
		nil,
		[]interface{}{
			nil,
			true,
			nil,
			nil,
			true,
			nil,
			[]interface{}{
				[]interface{}{[]interface{}{1, 1}, true},
				[]interface{}{[]interface{}{1, 2}, true},
				[]interface{}{[]interface{}{2, 1}, true},
				[]interface{}{[]interface{}{2, 2}, true},
				[]interface{}{[]interface{}{3, 1}, false},
			},
		},
		[]interface{}{
			1,
			[]interface{}{[]interface{}{albumId}, []interface{}{1, 2, 3}},
			[]interface{}{},
			nil,
			nil,
			[]interface{}{},
			[]interface{}{1},
			nil,
			nil,
			nil,
			[]interface{}{},
		},
		nil,
		[]interface{}{
			[]interface{}{
				[]interface{}{
					[]interface{}{2, shareWithUserId},
					nil,
					nil,
					nil,
					nil,
					[]interface{}{
						2,
						shareWithUserId,
						[]interface{}{
							nil,
							shareWithUserId,
							0,
							nil,
						},
					},
				},
			},
		},
		nil,
		nil,
		[]interface{}{1, 2, 3},
	}
	innerJsonString, err := json.Marshal(innerJson)
	if err != nil {
		return "", err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"SFKp8c",
				string(innerJsonString),
				nil,
				"generic",
			},
		},
	}
	innerJsonRes, err := doRequest(credentials, jsonReq)
	if err != nil {
		return "", err
	}

	sharedAlbumId, err := jsonparser.GetString(innerJsonRes, "[0]")
	if err != nil {
		return "", unexpectedResponse()
	}

	return sharedAlbumId, nil
}

// Delete Albums
func DeleteAlbum(credentials auth.CookieCredentials, albumId string) error {
	innerJson := []interface{}{
		[]interface{}{},
		[]interface{}{},
		[]interface{}{
			[]interface{}{
				albumId,
				nil,
				0,
			},
		},
	}
	innerJsonString, err := json.Marshal(innerJson)
	if err != nil {
		return err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"nV6Qv",
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

	return nil
}

// List all albums
func ListAllAlbums(credentials auth.CookieCredentials, cb func([]Album, error)) ([]Album, error) {
	var (
		nextPageToken interface{}
		allAlbums     []Album = []Album{}
		albums        []Album
		err           error
	)

	// Fetch all pages, 100 albums at once
	for {
		albums, nextPageToken, err = ListAlbums(credentials, nextPageToken, 100)
		if err != nil {
			return allAlbums, err
		}
		if cb != nil {
			cb(albums, err)
		}
		allAlbums = append(allAlbums, albums...)
		if nextPageToken == nil {
			// Return result
			return allAlbums, nil
		}
	}
}

// List albums by page
func ListAlbums(credentials auth.CookieCredentials, pageToken interface{}, pageSize int) ([]Album, interface{}, error) {
	innerJson := []interface{}{
		pageToken, // Page token
		nil,
		nil,
		nil,
		1,
		nil,
		nil,
		pageSize, // Page size
	}
	innerJsonString, err := json.Marshal(innerJson)
	if err != nil {
		return nil, nil, err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"Z5xsfc",
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

	var albums []Album = []Album{}
	if len(innerJson) > 2 {
		_, err = jsonparser.ArrayEach(innerJsonRes, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			var album Album
			album.AlbumId, err = jsonparser.GetString(value, "[0]")
			if err != nil {
				return
			}
			album.AlbumName, err = jsonparser.GetString(value, "[15]", "72930366", "[1]")
			if err != nil {
				return
			}
			album.MediaCount, err = jsonparser.GetInt(value, "[15]", "72930366", "[3]")
			if err != nil {
				return
			}
			albums = append(albums, album)
		}, "[0]")
		if err != nil {
			return albums, nil, unexpectedResponse()
		}
	}

	nextPageToken, err := jsonparser.GetString(innerJsonRes, "[1]")
	if err != nil {
		return albums, nil, nil
	}
	return albums, nextPageToken, nil
}
