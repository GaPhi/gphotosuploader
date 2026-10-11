package auth

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	// Endpoint used by Google web apps to renew the short-lived session cookies
	RotateCookiesUrl = "https://accounts.google.com/RotateCookies"

	// Cookie renewed by RotateCookiesUrl (__Secure-3PSIDTS gets the same value)
	rotatedCookieName = "__Secure-1PSIDTS"

	// Body sent by the browser (undocumented, taken from the HanaokaYuzu/Gemini-API implementation)
	rotateCookiesBody = `[000,"-0000000000000000000"]`
)

// RotateCookies asks Google for a new __Secure-1PSIDTS / __Secure-3PSIDTS pair, as a browser does while a
// Google page stays open. The new cookies are stored in the cookie jar: call SerializeToFile afterwards,
// otherwise the auth file keeps the previous values, which Google stops accepting some time after a rotation.
// Returns true if the cookie value changed.
// Google answers 429 when called more than about once a minute and 401 when the session is no longer valid.
func (c *CookieCredentials) RotateCookies() (bool, error) {
	before := c.cookieValue(rotatedCookieName)

	req, err := http.NewRequest("POST", RotateCookiesUrl, strings.NewReader(rotateCookiesBody))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://accounts.google.com")

	res, err := c.Client.Do(req)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	_, _ = io.Copy(io.Discard, res.Body)

	if res.StatusCode != http.StatusOK {
		return false, fmt.Errorf("auth: RotateCookies returned HTTP %v", res.StatusCode)
	}

	return c.cookieValue(rotatedCookieName) != before, nil
}

// Value of a cookie as it would be sent to Google Photos ("" if absent)
func (c *CookieCredentials) cookieValue(name string) string {
	cookiesUrl, _ := url.Parse(cookieDomainWithProtocol)
	for _, cookie := range c.Client.Jar.Cookies(cookiesUrl) {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}
