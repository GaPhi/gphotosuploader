package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	cookieDomainWithProtocol = "https://photos.google.com"
	cookiesDomain            = "photos.google.com"
	googleCookiesDomain      = ".google.com"
)

type PersistentParameters struct {
	UserId string `json:"userId"`

	// Time of the last accepted call to RotateCookiesUrl (zero if unknown)
	CookiesRotatedAt time.Time `json:"cookiesRotatedAt,omitzero"`
}

type RuntimeParameters struct {
	AtToken string
}

type CookieCredentials struct {
	Client               *http.Client
	PersistentParameters *PersistentParameters
	RuntimeParameters    *RuntimeParameters
}

type AuthFile struct {
	Cookies []*http.Cookie `json:"cookies"`

	// Parameters used by the requests (user id, etc ...)
	PersistentParameters *PersistentParameters `json:"persistantParameters"`
}

// Restore an CookieCredentials object from a JSON file
func NewCookieCredentialsFromFile(fileName string) (*CookieCredentials, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("auth: Can't open %v", fileName)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	return NewCookieCredentialsFromJson(file)
}

// Restore an CookieCredentials object from a JSON
func NewCookieCredentialsFromJson(in io.Reader) (*CookieCredentials, error) {
	authFile := AuthFile{}
	if err := json.NewDecoder(in).Decode(&authFile); err != nil {
		return nil, fmt.Errorf("auth: Can't read the JSON AuthFile (%v)", err)
	}

	return NewCookieCredentials(authFile.Cookies, authFile.PersistentParameters), nil
}

func NewCookieCredentials(cookies []*http.Cookie, parameters *PersistentParameters) *CookieCredentials {
	return &CookieCredentials{
		Client: &http.Client{
			Jar: createJarWithCookies(cookies),
		},
		PersistentParameters: parameters,
		RuntimeParameters:    &RuntimeParameters{},
	}
}

func createJarWithCookies(cookies []*http.Cookie) *cookiejar.Jar {
	jar, _ := cookiejar.New(nil)
	cookiesUrl, _ := url.Parse(cookieDomainWithProtocol)

	jar.SetCookies(cookiesUrl, cookies)

	return jar
}

// Serialize the CookieCredentials object into a JSON file, to be restored in the future using
// NewCookieCredentialsFromJsonFile.
// The file is replaced atomically (temporary file then rename) so that an interruption never leaves a
// truncated file. When the rename is not possible (file mounted alone in a container, directory not
// writable, ...), the file is overwritten in place.
func (c *CookieCredentials) SerializeToFile(fileName string) error {
	var content bytes.Buffer
	if err := c.Serialize(&content); err != nil {
		return err
	}

	// Replace the target of a symbolic link, not the link itself
	target := fileName
	if resolved, err := filepath.EvalSymlinks(fileName); err == nil {
		target = resolved
	}

	if err := replaceFile(target, content.Bytes()); err == nil {
		return nil
	}

	if err := os.WriteFile(target, content.Bytes(), 0600); err != nil {
		return fmt.Errorf("auth: Can't write the file %v (%v)", fileName, err)
	}
	return nil
}

// Write content into a temporary file next to fileName, then rename it as fileName
func replaceFile(fileName string, content []byte) error {
	mode := os.FileMode(0600)
	if info, err := os.Stat(fileName); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(filepath.Dir(fileName), "."+filepath.Base(fileName)+".*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()

	_, err = tmp.Write(content)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), mode)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), fileName)
	}
	return err
}

// Serialize the CookieCredentials object into a JSON object, to be restored in the future using
// NewCookieCredentialsFromJson
func (c *CookieCredentials) Serialize(out io.Writer) error {
	cookiesUrl, _ := url.Parse(cookieDomainWithProtocol)
	cookies := c.Client.Jar.Cookies(cookiesUrl)

	prepareCookiesForSerialization(cookies)

	return json.NewEncoder(out).Encode(&AuthFile{
		Cookies:              cookies,
		PersistentParameters: c.PersistentParameters,
	})
}

func prepareCookiesForSerialization(cookies []*http.Cookie) {
	for _, cookie := range cookies {
		if cookie.Name == "OTZ" {
			cookie.Domain = cookiesDomain
		} else {
			cookie.Domain = googleCookiesDomain
		}
		cookie.Path = "/"
	}
}
