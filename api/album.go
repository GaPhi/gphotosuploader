package api

import (
	"encoding/json"
	"errors"
	"github.com/buger/jsonparser"
	"github.com/simonedegiacomi/gphotosuploader/auth"
	"strings"
)

// Album represents an album
type Album struct {
	// Album identifier
	AlbumId string

	// Shared album identifier
	SharedAlbumId interface{}

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
		return "", unexpectedResponse(innerJsonRes)
	}

	return albumId, nil
}

func AlbumAddMediaItems(credentials auth.CookieCredentials, albumId string, items []string) error {
	innerJson := []interface{}{
		albumId,
		[]interface{}{
			2,
			nil,
			[]interface{}{
				[]interface{}{
					items,
				},
			},
			nil,
			nil,
			[]interface{}{},
			[]interface{}{
				1,
			},
			nil,
			nil,
			nil,
			[]interface{}{},
		},
	}
	innerJsonString, err := json.Marshal(innerJson)
	if err != nil {
		return err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"laUYf",
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

	// The image should now be part of the album
	return nil
}

func AlbumSortMediaItems(credentials auth.CookieCredentials, albumId string, kind int) error {
	var kindJson []interface{}
	switch kind {
	case 1: // Newest first
		kindJson = []interface{}{
			2,
			true,
		}
		break
	case 2: // Oldest first
		kindJson = []interface{}{
			2,
			false,
		}
		break
	case 3: // Last added first
		kindJson = []interface{}{
			3,
			true,
		}
		break
	default:
		return errors.New("bad album sort kind")
	}
	innerJson := []interface{}{
		albumId,
		[]interface{}{},
		4,
		nil,
		[]interface{}{},
		nil,
		nil,
		kindJson,
	}
	innerJsonString, err := json.Marshal(innerJson)
	if err != nil {
		return err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"QD9nKf",
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

	// The image should now be part of the album
	return nil
}

func createUserInterface(user string) interface{} {
	// User email (without dot before @)?
	if strings.Contains(user, "@") {
		return []interface{}{ // User identified by userEmail
			[]interface{}{
				6,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				user,
			},
			true,
			user,
			nil,
			nil,
			[]interface{}{
				5,
				user,
				[]interface{}{
					nil,
					nil,
					0,
					nil,
				},
			},
		}
	}

	// Consider it is a userId
	return []interface{}{ // User identified by userId
		[]interface{}{
			2,
			user,
		},
		nil,
		nil,
		nil,
		nil,
		[]interface{}{
			2,
			user,
			[]interface{}{
				nil,
				user,
				0,
				nil,
			},
		},
	}
}

// Share Album
func AlbumShareWithUser(credentials auth.CookieCredentials, albumId string, user string) (string, error) {
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
			[]interface{}{ // Users list
				createUserInterface(user),
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
		return "", unexpectedResponse(innerJsonRes)
	}

	return sharedAlbumId, nil
}

// Add a new user to a Album share
func AlbumShareAddUser(credentials auth.CookieCredentials, sharedAlbumId string, user string) error {
	innerJson := []interface{}{
		[]interface{}{
			sharedAlbumId,
		},
		[]interface{}{
			[]interface{}{ // Users list
				createUserInterface(user),
			}, // End of users list
			[]interface{}{},
		},
		[]interface{}{
			[]interface{}{},
			nil,
			nil,
			nil,
			[]interface{}{},
			[]interface{}{},
		},
		[]interface{}{
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			[]interface{}{},
			nil,
			nil,
			nil,
			nil,
			[]interface{}{},
		},
		nil,
		nil,
		false,
	}
	innerJsonString, err := json.Marshal(innerJson)
	if err != nil {
		return err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"NXNezb",
				string(innerJsonString),
				nil,
				"generic",
			},
		},
	}
	_, err = doRequest(credentials, jsonReq)
	// If already shared : no error
	// If album owner : no error
	// If user is unknown : unexpected JSON response structure: [["wrb.fr","NXNezb",null,null,null,[3],"generic"],["di",132],["af.httprm",132,"8627835294917737988",8]]
	if err != nil {
		return err
	}

	return nil
}

// Delete Albums
// albumId: Own albumId (AF1QipP5CHoTNeAsjAdNQDbfaWTI0A2oJp_er5PSNSFs)
// sharedAlbumId: Shared album Id (AF1QipN4Q7SPvfG2agzCI_ZTH2Hp7zNTGSOcH4MhUuCmNHxKr1JfU3Uz-vg7heZ2z195PA)
func DeleteAlbum(credentials auth.CookieCredentials, albumId string, sharedAlbumId interface{}) error {
	innerJson := []interface{}{
		[]interface{}{},
		[]interface{}{},
		[]interface{}{
			[]interface{}{
				albumId,
				sharedAlbumId,
				0, // TODO Find Integer (528 for instance)
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
		allAlbums     = []Album{}
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
		if nextPageToken == nil || nextPageToken == "" {
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

	albums := []Album{}
	_, _ = jsonparser.ArrayEach(innerJsonRes, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		var album Album
		album.SharedAlbumId, err = jsonparser.GetString(value, "[0]")
		if err != nil {
			return
		}
		album.AlbumName, err = jsonparser.GetString(value, "[8]", "72930366", "[1]")
		if err != nil {
			return
		}
		album.MediaCount, err = jsonparser.GetInt(value, "[8]", "72930366", "[3]")
		if err != nil {
			return
		}
		album.AlbumId, err = jsonparser.GetString(value, "[8]", "72930366", "[8]")
		if err != nil {
			return
		}
		albums = append(albums, album)
	}, "[0]")

	nextPageToken, err := jsonparser.GetString(innerJsonRes, "[1]")
	if err != nil {
		return albums, nil, nil
	}
	return albums, nextPageToken, nil
}

// Delete empty albums
func DeleteEmptyAlbums(credentials auth.CookieCredentials) ([]Album, []Album, []Album, error) {
	var deleted, notDeleted []Album
	albums, err := ListAllAlbums(credentials, func(albumsPart []Album, err error) {
		// No album?
		if len(albumsPart) == 0 {
			return
		}

		// Delete empty albums
		for _, album := range albumsPart {
			if album.MediaCount == 0 { // TODO: Only if owned (not shared album?)
				err = DeleteAlbum(credentials, album.AlbumId, album.SharedAlbumId)
				if err != nil {
					notDeleted = append(notDeleted, album)
				} else {
					deleted = append(deleted, album)
				}
			}
		}
	})
	return albums, deleted, notDeleted, err
}
