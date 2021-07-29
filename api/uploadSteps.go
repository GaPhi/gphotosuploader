package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/buger/jsonparser"
)

const (
	// NewUploadURL : Url to which send the request to get a new url to upload a new image
	NewUploadURL = "https://photos.google.com/_/upload/uploadmedia/rupio/interactive?authuser=2"
)

// Method that send a request with the file name and size to generate an upload url.
func (u *Upload) requestUploadURL() error {
	credentialsPersistentParameters := u.Credentials.PersistentParameters
	if credentialsPersistentParameters == nil {
		return fmt.Errorf("failed getting Credentials persistent parameters. Not set")
	}

	// Prepare json request
	jsonReq := RequestUploadURL{
		ProtocolVersion: "0.8",
		CreateSessionRequest: CreateSessionRequest{
			Fields: []interface{}{
				ExternalField{
					External: ExternalFieldObject{
						Name:     "file",
						Filename: u.Options.Name,
						Size:     u.Options.FileSize,
					},
				},

				// Additional fields
				InlinedField{
					Inlined: InlinedFieldObject{
						Name:        "auto_create_album",
						Content:     "camera_sync.active",
						ContentType: "text/plain",
					},
				},
				InlinedField{
					Inlined: InlinedFieldObject{
						Name:        "auto_downsize",
						Content:     "true",
						ContentType: "text/plain",
					},
				},
				InlinedField{
					Inlined: InlinedFieldObject{
						Name:        "storage_policy",
						Content:     "use_manual_setting",
						ContentType: "text/plain",
					},
				},
				InlinedField{
					Inlined: InlinedFieldObject{
						Name:        "disable_asbe_notification",
						Content:     "true",
						ContentType: "text/plain",
					},
				},
				InlinedField{
					Inlined: InlinedFieldObject{
						Name:        "client",
						Content:     "photoweb",
						ContentType: "text/plain",
					},
				},
				InlinedField{
					Inlined: InlinedFieldObject{
						Name:        "effective_id",
						Content:     credentialsPersistentParameters.UserId,
						ContentType: "text/plain",
					},
				},
				InlinedField{
					Inlined: InlinedFieldObject{
						Name:        "owner_name",
						Content:     credentialsPersistentParameters.UserId,
						ContentType: "text/plain",
					},
				},
			},
		},
	}

	// Create http request
	jsonStr, err := json.Marshal(jsonReq)
	req, err := http.NewRequest("POST", NewUploadURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		return errors.New(fmt.Sprintf("Can't create upload URL request: %v", err.Error()))
	}

	// Add headers for the request
	req.Header.Add("x-guploader-client-info", "mechanism=scotty xhr resumable; clientVersion=156351954")

	// Make the request
	res, err := u.Credentials.Client.Do(req)
	if err != nil {
		return errors.New(fmt.Sprintf("Error during the request to get the upload URL: %v", err.Error()))
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	// Parse the json response
	jsonResponse, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return responseReadingError()
	}

	u.url, err = jsonparser.GetString(jsonResponse, "sessionStatus", "externalFieldTransfers", "[0]", "putInfo", "url")
	return err
}

// This method upload the file to the URL received from requestUploadUrl.
// When the upload is completed, the method updates the base64UploadToken field
func (u *Upload) uploadFile() (token string, err error) {
	if u.url == "" {
		return "", errors.New("the url field is empty, make sure to call requestUploadUrl first")
	}

	// Create the request
	req, err := http.NewRequest("POST", u.url, u.Options.Stream)
	if err != nil {
		return "", fmt.Errorf("can't create upload URL request: %v", err.Error())
	}

	// Prepare request headers
	req.Header.Add("content-type", "application/octet-stream")
	req.Header.Add("content-length", fmt.Sprintf("%v", u.Options.FileSize))
	req.Header.Add("X-HTTP-Method-Override", "PUT")

	// Upload the image
	res, err := u.Credentials.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("can't upload the image, got: %v", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	// Parse the response
	jsonRes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", nil
	}

	return jsonparser.GetString(jsonRes, "sessionStatus", "additionalInfo", "uploader_service.GoogleRupioAdditionalInfo", "completionInfo", "customerSpecificInfo", "upload_token_base64")
}

// Request that enables the image once it gets uploaded
func (u *Upload) enablePhoto(uploadTokenBase64 string) (enabledUrl string, err error) {
	innerJson := []interface{}{
		[]interface{}{
			[]interface{}{
				uploadTokenBase64,
				u.Options.Name,
				u.Options.Timestamp,
			},
		},
	}
	innerJsonStr, err := json.Marshal(innerJson)
	if err != nil {
		return "", err
	}
	jsonReq := []interface{}{
		[]interface{}{
			[]interface{}{
				"mdpdU",
				string(innerJsonStr),
				nil,
				"generic",
			},
		},
	}
	innerJsonRes, err := doRequest(u.Credentials, jsonReq)
	if err != nil {
		return "", err
	}

	eUrl, err := jsonparser.GetString(innerJsonRes, "[0]", "[0]", "[1]", "[1]", "[0]")
	if err != nil {
		return "", unexpectedResponse(innerJsonRes)
	}
	u.idToMoveIntoAlbum, err = jsonparser.GetString(innerJsonRes, "[0]", "[0]", "[1]", "[0]")
	if err != nil {
		return "", unexpectedResponse(innerJsonRes)
	}

	return eUrl, nil
}

// This method add the image to an existing album given the id
func (u *Upload) moveToAlbum(albumId string) error {
	if u.idToMoveIntoAlbum == "" {
		return errors.New(fmt.Sprint("can't move image to album without the enabled image id"))
	}

	innerJson := []interface{}{
		albumId,
		[]interface{}{
			2,
			nil,
			[]interface{}{
				[]interface{}{
					[]interface{}{
						u.idToMoveIntoAlbum,
					},
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
	_, err = doRequest(u.Credentials, jsonReq)
	if err != nil {
		return err
	}

	// The image should now be part of the album
	return nil
}

// Create Album
func (u *Upload) createAlbum(albumName string) (string, error) {
	if u.idToMoveIntoAlbum == "" {
		return "", errors.New(fmt.Sprint("can't create album without the enabled image id"))
	}

	innerJson := []interface{}{
		albumName,
		nil,
		1,
		[]interface{}{
			[]interface{}{
				[]interface{}{
					u.idToMoveIntoAlbum,
				},
			},
		},
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
	innerJsonRes, err := doRequest(u.Credentials, jsonReq)
	if err != nil {
		return "", err
	}

	albumId, err := jsonparser.GetString(innerJsonRes, "[0]", "[0]")
	if err != nil {
		return "", unexpectedResponse(innerJsonRes)
	}

	return albumId, nil
}
