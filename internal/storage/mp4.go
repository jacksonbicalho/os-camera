package storage

import (
	"encoding/binary"
	"io"
	"os"
)

// isValidMP4 reports whether the file at path contains a moov atom,
// which indicates a properly closed MP4. It only reads atom headers
// and seeks past payloads, so it is O(number of top-level atoms).
func isValidMP4(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	var buf [8]byte
	for {
		if _, err := io.ReadFull(f, buf[:]); err != nil {
			return false
		}
		size := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
		typ := string(buf[4:8])

		if typ == "moov" {
			return true
		}

		var skip int64
		switch {
		case size == 0:
			return false // atom extends to EOF; no moov found
		case size == 1:
			// Extended 64-bit size: next 8 bytes hold the real size.
			var ext [8]byte
			if _, err := io.ReadFull(f, ext[:]); err != nil {
				return false
			}
			actual := binary.BigEndian.Uint64(ext[:])
			skip = int64(actual) - 16 // already read 16 bytes (8 hdr + 8 ext)
		default:
			skip = int64(size) - 8
		}

		if skip < 0 {
			return false
		}
		if skip > 0 {
			if _, err := f.Seek(skip, io.SeekCurrent); err != nil {
				return false
			}
		}
	}
}
