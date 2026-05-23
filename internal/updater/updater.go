package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// DefaultAPIURL lists releases sorted by creation date; the first item is the most recent
// regardless of whether it is a pre-release (unlike the /releases/latest endpoint, which
// returns 404 when all releases are pre-releases).
const DefaultAPIURL = "https://api.github.com/repos/jacksonbicalho/camera/releases?per_page=1"

type UpdateInfo struct {
	Current         string            `json:"current"`
	Latest          string            `json:"latest"`
	UpdateAvailable bool              `json:"update_available"`
	ChangelogURL    string            `json:"changelog_url"`
	Assets          map[string]string `json:"assets"`
	ChecksumsURL    string            `json:"checksums_url"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckLatest queries apiURL for the latest release and compares with current.
// Pass DefaultAPIURL in production; pass a test server URL in tests.
func CheckLatest(current, apiURL string) (UpdateInfo, error) {
	info := UpdateInfo{
		Current: current,
		Assets:  map[string]string{},
	}

	resp, err := httpClient.Get(apiURL) //nolint:noctx
	if err != nil {
		return info, fmt.Errorf("updater: fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("updater: GitHub API returned %d", resp.StatusCode)
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return info, fmt.Errorf("updater: decode response: %w", err)
	}
	if len(releases) == 0 {
		return info, nil
	}
	rel := releases[0]

	info.Latest = rel.TagName
	info.ChangelogURL = rel.HTMLURL

	for _, a := range rel.Assets {
		switch {
		case a.Name == "checksums.txt":
			info.ChecksumsURL = a.BrowserDownloadURL
		case strings.HasPrefix(a.Name, "camera-linux-"):
			arch := strings.TrimPrefix(a.Name, "camera-linux-")
			info.Assets[arch] = a.BrowserDownloadURL
		}
	}

	// dev builds and identical versions are never flagged as outdated
	if current != "dev" && rel.TagName != "" && rel.TagName != current {
		info.UpdateAvailable = newerThan(rel.TagName, current)
	}

	return info, nil
}

// IsDocker reports whether the process is running inside a Docker container.
func IsDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// newerThan returns true if candidate is a higher semver than base.
// Handles vX.Y.Z and vX.Y.Z-beta.N formats.
func newerThan(candidate, base string) bool {
	cv := parseVer(candidate)
	bv := parseVer(base)
	for i := range cv {
		if cv[i] > bv[i] {
			return true
		}
		if cv[i] < bv[i] {
			return false
		}
	}
	return false
}

func parseVer(v string) [4]int {
	v = strings.TrimPrefix(v, "v")
	// strip pre-release suffix: -beta.N → keep N as 4th component
	var pre int
	if idx := strings.Index(v, "-"); idx >= 0 {
		fmt.Sscanf(v[idx+1:], "%*[a-z].%d", &pre) //nolint
		v = v[:idx]
	}
	var maj, min, pat int
	fmt.Sscanf(v, "%d.%d.%d", &maj, &min, &pat) //nolint
	return [4]int{maj, min, pat, pre}
}
