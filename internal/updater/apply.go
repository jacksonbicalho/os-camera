package updater

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// Apply downloads the binary for the current architecture from info, verifies
// its checksum, replaces the running executable, and re-execs into the new binary.
// On Docker it returns ErrDocker without modifying anything.
func Apply(info UpdateInfo) error {
	if IsDocker() {
		return ErrDocker
	}

	arch := runtime.GOARCH
	downloadURL, ok := info.Assets[arch]
	if !ok {
		return fmt.Errorf("updater: no asset for arch %s in release %s", arch, info.Latest)
	}

	// download binary to temp file on the same filesystem as the executable
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("updater: resolve executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("updater: eval symlinks: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(exe), ".camera-update-*")
	if err != nil {
		return fmt.Errorf("updater: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { os.Remove(tmpPath) }()

	if err := download(downloadURL, tmpFile); err != nil {
		tmpFile.Close()
		return fmt.Errorf("updater: download binary: %w", err)
	}
	tmpFile.Close()

	// verify checksum if available
	if info.ChecksumsURL != "" {
		assetName := "camera-linux-" + arch
		if err := verifyChecksum(tmpPath, info.ChecksumsURL, assetName); err != nil {
			return fmt.Errorf("updater: checksum mismatch: %w", err)
		}
	}

	// make executable
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("updater: chmod: %w", err)
	}

	// atomic replace
	if err := os.Rename(tmpPath, exe); err != nil {
		return fmt.Errorf("updater: replace binary: %w", err)
	}

	// re-exec into new binary — replaces the current process
	return syscall.Exec(exe, os.Args, os.Environ())
}

var ErrDocker = fmt.Errorf("updater: running inside Docker — pull a new image to update")

func download(url string, dst io.Writer) error {
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	_, err = io.Copy(dst, resp.Body)
	return err
}

func verifyChecksum(path, checksumsURL, assetName string) error {
	resp, err := http.Get(checksumsURL) //nolint:noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var expected string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			expected = fields[0]
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("checksum for %s not found", assetName)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != expected {
		return fmt.Errorf("got %s, want %s", got, expected)
	}
	return nil
}
