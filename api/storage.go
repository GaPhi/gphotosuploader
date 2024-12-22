package api

import (
	"encoding/json"

	"github.com/GaPhi/gphotosuploader/auth"
	"github.com/buger/jsonparser"
)

// Create Album
func QueryStorage(credentials auth.CookieCredentials) (int64, int64, error) {
	innerJson := []interface{}{
		[]interface{}{
			[]interface{}{
				[]interface{}{
					nil,
					string(credentials.PersistentParameters.UserId),
				},
			},
			[]interface{}{
				nil,
				[]interface{}{},
				[]interface{}{
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					nil,
					[]interface{}{},
				},
				[]interface{}{
					nil,
					nil,
					[]interface{}{
						[]interface{}{},
					},
				},
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				nil,
				[]interface{}{},
			},
		},
	}
	innerJsonString, err := json.Marshal(innerJson)
	if err != nil {
		return -1, -1, err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"eNG3nf",
				string(innerJsonString),
				nil,
				"generic",
			},
		},
	}
	innerJsonRes, err := doRequest(credentials, jsonReq)
	if err != nil {
		return -1, -1, err
	}

	used, err := jsonparser.GetInt(innerJsonRes, "[0]", "[1]", "[0]", "[7]", "[0]")
	if err != nil {
		return -1, -1, unexpectedResponse(innerJsonRes)
	}
	total, err := jsonparser.GetInt(innerJsonRes, "[0]", "[1]", "[0]", "[7]", "[1]")
	if err != nil {
		return -1, -1, unexpectedResponse(innerJsonRes)
	}

	return used, total, nil
}
