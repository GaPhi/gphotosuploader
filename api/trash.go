package api

import (
	"encoding/json"
	"github.com/simonedegiacomi/gphotosuploader/auth"
)

// Empty trash
func EmptyTrash(credentials auth.CookieCredentials) error {
	innerJson := []interface{}{
		[]interface{}{},
	}
	innerJsonString, err := json.Marshal(innerJson)
	if err != nil {
		return err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"vzCSKc",
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
