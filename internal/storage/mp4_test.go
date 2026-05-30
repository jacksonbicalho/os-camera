package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// minimalValidMP4 returns the bytes of a minimal but structurally valid MP4 file:
// ftyp(24) + mdat(8, empty) + moov(8, empty).
func minimalValidMP4() []byte {
	return []byte{
		// ftyp: size=24
		0, 0, 0, 24, 'f', 't', 'y', 'p',
		'i', 's', 'o', 'm', 0, 0, 0, 0,
		'i', 's', 'o', 'm', 'm', 'p', '4', '1',
		// mdat: size=8, no payload
		0, 0, 0, 8, 'm', 'd', 'a', 't',
		// moov: size=8, no payload
		0, 0, 0, 8, 'm', 'o', 'o', 'v',
	}
}

func TestIsValidMP4_ValidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ok.mp4")
	if err := os.WriteFile(path, minimalValidMP4(), 0644); err != nil {
		t.Fatal(err)
	}
	if !isValidMP4(path) {
		t.Error("expected valid MP4 to return true")
	}
}

func TestIsValidMP4_CorruptFile_RandomBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.mp4")
	if err := os.WriteFile(path, []byte("this is not an mp4 file"), 0644); err != nil {
		t.Fatal(err)
	}
	if isValidMP4(path) {
		t.Error("expected corrupt file (random bytes) to return false")
	}
}

func TestIsValidMP4_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.mp4")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if isValidMP4(path) {
		t.Error("expected empty file to return false")
	}
}

func TestIsValidMP4_MissingFile(t *testing.T) {
	if isValidMP4(filepath.Join(t.TempDir(), "nonexistent.mp4")) {
		t.Error("expected missing file to return false")
	}
}

func TestIsValidMP4_NoMoovAtom(t *testing.T) {
	// ftyp + mdat only — no moov
	b := []byte{
		0, 0, 0, 24, 'f', 't', 'y', 'p',
		'i', 's', 'o', 'm', 0, 0, 0, 0,
		'i', 's', 'o', 'm', 'm', 'p', '4', '1',
		0, 0, 0, 8, 'm', 'd', 'a', 't',
	}
	path := filepath.Join(t.TempDir(), "nomoov.mp4")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	if isValidMP4(path) {
		t.Error("expected file without moov atom to return false")
	}
}

func TestIsValidMP4_ExtendedSizeAtom(t *testing.T) {
	// ftyp with extended 64-bit size (size field = 1), followed by moov
	// ftyp total = 8 (hdr) + 8 (ext-size) + 12 (content) = 28 bytes
	b := make([]byte, 0, 36)
	b = append(b,
		// ftyp header: size=1 (extended), type='ftyp'
		0, 0, 0, 1, 'f', 't', 'y', 'p',
		// extended size = 28
		0, 0, 0, 0, 0, 0, 0, 28,
		// content: brand + version + compat
		'i', 's', 'o', 'm', 0, 0, 0, 0, 'i', 's', 'o', 'm',
	)
	// moov
	b = append(b, 0, 0, 0, 8, 'm', 'o', 'o', 'v')

	path := filepath.Join(t.TempDir(), "extended.mp4")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	if !isValidMP4(path) {
		t.Error("expected MP4 with extended-size ftyp atom to return true")
	}
}
