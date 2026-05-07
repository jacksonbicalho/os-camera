//go:build windows

package server

func diskStats(path string) (total, free int64) {
	return 0, 0
}
