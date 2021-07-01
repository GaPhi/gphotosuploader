package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/simonedegiacomi/gphotosuploader/auth"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const (
	// Url to post requests
	batchExecuteUrl = "https://photos.google.com/u/0/_/PhotosUi/data/batchexecute"
)

var (
	// JSON response header (cannot be an actual constant in Go)
	jsonHeader = []byte{')', ']', '}', '\'', '\n', '\n'}
)

func responseReadingError() error {
	return fmt.Errorf("can't read response")
}

func unexpectedResponse(jsonRes []byte) error {
	return fmt.Errorf("unexpected JSON response structure: %v", jsonRes)
}

// doRequest posts up to 3 times the request
// returns innerJson array of bytes if success
// returns jsonRes array of bytes in case of unexpectedResponse
// returns nil,error in case of any other error
func doRequest(credentials auth.CookieCredentials, jsonReq []interface{}) ([]byte, error) {
	jsonString, err := json.Marshal(jsonReq)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Add("f.req", string(jsonString))
	form.Add("at", credentials.RuntimeParameters.AtToken)

	var jsonRes []byte
	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("POST", batchExecuteUrl, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, fmt.Errorf("can't create the request: %v", err.Error())
		}
		req.Header.Add("content-type", "application/x-www-form-urlencoded;charset=UTF-8")

		res, err := credentials.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error sending the request: %v", err.Error())
		}

		// Read the response as a string
		jsonRes, err = ioutil.ReadAll(res.Body)
		_ = res.Body.Close()
		if err != nil {
			return nil, responseReadingError()
		}

		// Valid response?
		if bytes.Compare(jsonRes[0:len(jsonHeader)], jsonHeader) == 0 {
			// Skip first characters
			innerJsonRes, err := jsonparser.GetString(jsonRes[len(jsonHeader):], "[0]", "[2]")
			if err != nil {
				return jsonRes, unexpectedResponse(jsonRes)
			}
			return []byte(innerJsonRes), nil
		}
	}

	// Cannot get result
	return jsonRes, unexpectedResponse(jsonRes)
}
