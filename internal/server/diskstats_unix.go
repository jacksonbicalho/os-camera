//go:build !windows

package server

import "syscall"

func diskStats(path string) (total, free int64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	return int64(stat.Blocks) * int64(stat.Frsize),
		int64(stat.Bavail) * int64(stat.Frsize)
}
