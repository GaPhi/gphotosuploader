package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/GaPhi/gphotosuploader/auth"
	"io"
	"log"
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
  LogRequests bool
)

func responseReadingError() error {
	return fmt.Errorf("can't read response")
}

func unexpectedResponse(jsonRes []byte) error {
	return fmt.Errorf("unexpected JSON response structure: %v", string(jsonRes))
}

func responseFailure(errTxt string, jsonRes []byte) error {
	return fmt.Errorf("failure: %v (%v)", errTxt, string(jsonRes))
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
	if LogRequests {
		log.Printf("Request: %v\n", string(jsonString))
  }
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
		jsonRes, err = io.ReadAll(res.Body)
		_ = res.Body.Close()
		if err != nil {
			return nil, responseReadingError()
		}
		if LogRequests {
			log.Printf("Response: %v\n", string(jsonRes))
		}

		// Valid response?
		if bytes.Equal(jsonRes[0:len(jsonHeader)], jsonHeader) {
			// Skip first characters
			jsonRes = jsonRes[len(jsonHeader):]
			innerJsonRes, err := jsonparser.GetString(jsonRes, "[0]", "[2]")
			if err == nil {
				return []byte(innerJsonRes), nil
			}
			// Example: [["wrb.fr","mdpdU",null,null,null,[8,null,[["type.googleapis.com/social.frontend.photos.data.PhotosCreateMediaItemsFailure",[1,[16550041816,16106127360,null,true,[[3]],0]]]]],"generic"],["di",369],["af.httprm",368,"4309186227030082757",19]]
			failure, err := jsonparser.GetString(jsonRes, "[5]", "[2]", "[0]", "[0]")
			if err == nil {
				return jsonRes, responseFailure(failure, jsonRes)
			}
		}
	}

	// Cannot get result
	return jsonRes, unexpectedResponse(jsonRes)
}
