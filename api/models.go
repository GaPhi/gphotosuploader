package api

// Structure of the JSON object that it's sent to request a new url to upload a new photo
type RequestUploadURL struct {
	ProtocolVersion      string               `json:"protocolVersion"`
	CreateSessionRequest CreateSessionRequest `json:"createSessionRequest"`
}

// Inner object of the request to get a new url to upload a photo.
type CreateSessionRequest struct {
	// The fields array is a slice that should contain only ExternalField or InternalField structs
	Fields []interface{} `json:"fields"`
}

// Possible field for the Fields slice in the CreateSessionRequest struct
type ExternalField struct {
	External ExternalFieldObject `json:"external"`
}

// Possible field for the Fields slice in the CreateSessionRequest struct
type InlinedField struct {
	Inlined InlinedFieldObject `json:"inlined"`
}

// Describes the file that need to be uploaded. This objects should be contained in a ExternalField
type ExternalFieldObject struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// Used to define parameters of the upload. This object should be contained in a InternalField
type InlinedFieldObject struct {
	Name        string `json:"name"`
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
}

type ApiTokenContainer struct {
	Token string `json:"SNlM0e"`
}
