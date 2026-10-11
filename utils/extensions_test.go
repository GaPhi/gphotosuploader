package utils

import "testing"

func TestIsExtensionSupportedIgnoresCase(t *testing.T) {
	for _, ext := range []string{".jpg", ".JPG", ".Mov", ".MTS"} {
		if !isExtensionSupported(ext) {
			t.Errorf("%v should be supported", ext)
		}
	}
	if isExtensionSupported(".txt") {
		t.Error(".txt should not be supported")
	}
}

func TestIsIgnoredPath(t *testing.T) {
	ignored := []string{"/volume1/photo/@eaDir", "/volume1/photo/@eaDir/IMG.JPG/SYNOPHOTO_THUMB_M.jpg"}
	kept := []string{"/volume1/photo/IMG.JPG", "/volume1/photo/my@eaDir/IMG.JPG"}
	for _, p := range ignored {
		if !IsIgnoredPath(p) {
			t.Errorf("%v should be ignored", p)
		}
	}
	for _, p := range kept {
		if IsIgnoredPath(p) {
			t.Errorf("%v should not be ignored", p)
		}
	}
}
