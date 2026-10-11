package auth

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Same shape as an auth file written by SerializeToFile
const testAuthFile = `{"cookies":[` +
	`{"Name":"SID","Value":"sid","Quoted":false,"Path":"/","Domain":".google.com","Expires":"0001-01-01T00:00:00Z","RawExpires":"","MaxAge":0,"Secure":false,"HttpOnly":false,"SameSite":0,"Partitioned":false,"Raw":"","Unparsed":null},` +
	`{"Name":"__Secure-1PSID","Value":"psid","Quoted":false,"Path":"/","Domain":".google.com","Expires":"0001-01-01T00:00:00Z","RawExpires":"","MaxAge":0,"Secure":false,"HttpOnly":false,"SameSite":0,"Partitioned":false,"Raw":"","Unparsed":null},` +
	`{"Name":"OTZ","Value":"otz","Quoted":false,"Path":"/","Domain":"photos.google.com","Expires":"0001-01-01T00:00:00Z","RawExpires":"","MaxAge":0,"Secure":false,"HttpOnly":false,"SameSite":0,"Partitioned":false,"Raw":"","Unparsed":null},` +
	`{"Name":"__Secure-1PSIDTS","Value":"old","Quoted":false,"Path":"/","Domain":".google.com","Expires":"0001-01-01T00:00:00Z","RawExpires":"","MaxAge":0,"Secure":false,"HttpOnly":false,"SameSite":0,"Partitioned":false,"Raw":"","Unparsed":null},` +
	`{"Name":"__Secure-3PSIDTS","Value":"old","Quoted":false,"Path":"/","Domain":".google.com","Expires":"0001-01-01T00:00:00Z","RawExpires":"","MaxAge":0,"Secure":false,"HttpOnly":false,"SameSite":0,"Partitioned":false,"Raw":"","Unparsed":null}` +
	`],"persistantParameters":{"userId":"42"}}`

type fakeGoogle struct {
	t      *testing.T
	status int
	seen   *http.Request
	body   string
}

func (f *fakeGoogle) RoundTrip(req *http.Request) (*http.Response, error) {
	f.seen = req
	b, _ := io.ReadAll(req.Body)
	f.body = string(b)
	h := http.Header{}
	if f.status == http.StatusOK {
		h.Add("Set-Cookie", "__Secure-1PSIDTS=new; Domain=.google.com; Path=/; Expires=Mon, 11-Oct-2027 08:00:00 GMT; Secure; HttpOnly; Priority=HIGH")
		h.Add("Set-Cookie", "__Secure-3PSIDTS=new; Domain=.google.com; Path=/; Expires=Mon, 11-Oct-2027 08:00:00 GMT; Secure; HttpOnly; Priority=HIGH; SameSite=none")
	}
	return &http.Response{StatusCode: f.status, Header: h, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
}

func TestRotateCookies(t *testing.T) {
	c, err := NewCookieCredentialsFromJson(strings.NewReader(testAuthFile))
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeGoogle{t: t, status: http.StatusOK}
	c.Client.Transport = fake

	rotated, err := c.RotateCookies()
	if err != nil || !rotated {
		t.Fatalf("rotated=%v err=%v", rotated, err)
	}
	if fake.seen.Method != "POST" || fake.seen.URL.String() != RotateCookiesUrl || fake.body != rotateCookiesBody {
		t.Fatalf("unexpected request %v %v %q", fake.seen.Method, fake.seen.URL, fake.body)
	}
	sent := fake.seen.Header.Get("Cookie")
	for _, want := range []string{"SID=sid", "__Secure-1PSID=psid", "__Secure-1PSIDTS=old"} {
		if !strings.Contains(sent, want) {
			t.Errorf("cookie %q not sent to accounts.google.com: %q", want, sent)
		}
	}
	if strings.Contains(sent, "OTZ=") {
		t.Errorf("photos.google.com cookie sent to accounts.google.com: %q", sent)
	}

	// The new values must reach the auth file, the others must survive
	out := &bytes.Buffer{}
	if err := c.Serialize(out); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewCookieCredentialsFromJson(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"__Secure-1PSIDTS": "new", "__Secure-3PSIDTS": "new", "SID": "sid", "__Secure-1PSID": "psid", "OTZ": "otz"} {
		if got := reloaded.cookieValue(name); got != want {
			t.Errorf("%v = %q after reload, want %q", name, got, want)
		}
	}
	if strings.Count(out.String(), `"__Secure-1PSIDTS"`) != 1 {
		t.Errorf("duplicated cookie in auth file: %v", out.String())
	}
	if reloaded.PersistentParameters.UserId != "42" {
		t.Errorf("userId lost")
	}
}

func TestRotateCookiesRefused(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusTooManyRequests} {
		c, _ := NewCookieCredentialsFromJson(strings.NewReader(testAuthFile))
		c.Client.Transport = &fakeGoogle{t: t, status: status}
		rotated, err := c.RotateCookies()
		if err == nil || rotated {
			t.Errorf("status %v: rotated=%v err=%v", status, rotated, err)
		}
		if c.cookieValue("__Secure-1PSIDTS") != "old" {
			t.Errorf("status %v: cookie changed", status)
		}
	}
}

func TestRotationTimeIsRemembered(t *testing.T) {
	c, _ := NewCookieCredentialsFromJson(strings.NewReader(testAuthFile))
	if !c.LastRotation().IsZero() || c.RotatedWithin(time.Hour) {
		t.Fatalf("auth file without rotation time: last=%v", c.LastRotation())
	}

	// Never written while unknown
	out := &bytes.Buffer{}
	_ = c.Serialize(out)
	if strings.Contains(out.String(), "cookiesRotatedAt") {
		t.Errorf("zero rotation time written: %v", out.String())
	}

	c.Client.Transport = &fakeGoogle{t: t, status: http.StatusOK}
	if _, err := c.RotateCookies(); err != nil {
		t.Fatal(err)
	}
	if !c.RotatedWithin(time.Hour) {
		t.Fatalf("rotation not remembered: last=%v", c.LastRotation())
	}

	// Survives a save and a reload
	out.Reset()
	_ = c.Serialize(out)
	reloaded, err := NewCookieCredentialsFromJson(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.LastRotation().Equal(c.LastRotation()) || !reloaded.RotatedWithin(time.Hour) {
		t.Errorf("rotation time lost: %v != %v", reloaded.LastRotation(), c.LastRotation())
	}
	if reloaded.PersistentParameters.UserId != "42" {
		t.Errorf("userId lost")
	}
}

func TestRotationTimeNotSetWhenRefused(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusTooManyRequests} {
		c, _ := NewCookieCredentialsFromJson(strings.NewReader(testAuthFile))
		c.Client.Transport = &fakeGoogle{t: t, status: status}
		_, _ = c.RotateCookies()
		if !c.LastRotation().IsZero() {
			t.Errorf("status %v: rotation time set", status)
		}
	}
}

func TestRotatedWithin(t *testing.T) {
	for _, tc := range []struct {
		ago  time.Duration
		want bool
	}{
		{time.Minute, true},
		{59 * time.Minute, true},
		{61 * time.Minute, false},
		{-time.Hour, false}, // in the future: clock changed
	} {
		c := &CookieCredentials{PersistentParameters: &PersistentParameters{CookiesRotatedAt: time.Now().Add(-tc.ago)}}
		if got := c.RotatedWithin(time.Hour); got != tc.want {
			t.Errorf("rotated %v ago: got %v, want %v", tc.ago, got, tc.want)
		}
	}
	if (&CookieCredentials{}).RotatedWithin(time.Hour) {
		t.Errorf("no persistent parameters: got true")
	}
}
