package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSerializeToFile(t *testing.T) {
	c, err := NewCookieCredentialsFromJson(strings.NewReader(testAuthFile))
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "real.json")
	if err := os.WriteFile(target, []byte("old"), 0640); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "auth.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := c.SerializeToFile(link); err != nil {
		t.Fatal(err)
	}

	// The link is kept and its target updated, with the same permissions
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symbolic link replaced: %v %v", info, err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0640 {
		t.Errorf("mode = %v, want 0640", info.Mode().Perm())
	}
	restored, err := NewCookieCredentialsFromFile(link)
	if err != nil {
		t.Fatal(err)
	}
	if restored.cookieValue("__Secure-1PSIDTS") != "old" || restored.PersistentParameters.UserId != "42" {
		t.Errorf("unexpected content after restore")
	}

	// No temporary file left
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("unexpected files in %v: %v", dir, entries)
	}
}

// Directory not writable (as a file mounted alone in a container): the file is overwritten in place
func TestSerializeToFileInPlace(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permissions are not enforced for root")
	}
	c, err := NewCookieCredentialsFromJson(strings.NewReader(testAuthFile))
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	fileName := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(fileName, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chmod(dir, 0700)
	}()

	if err := c.SerializeToFile(fileName); err != nil {
		t.Fatal(err)
	}
	if _, err := NewCookieCredentialsFromFile(fileName); err != nil {
		t.Fatal(err)
	}
}
