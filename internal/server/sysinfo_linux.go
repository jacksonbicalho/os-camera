//go:build linux

package server

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type cpuTracker struct {
	mu       sync.Mutex
	lastJiff uint64
	lastAt   time.Time
}

func (t *cpuTracker) percent() float64 {
	jiff, err := readCPUJiffies()
	if err != nil {
		return -1
	}
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastAt.IsZero() {
		t.lastJiff = jiff
		t.lastAt = now
		return -1
	}
	elapsed := now.Sub(t.lastAt).Seconds()
	delta := jiff - t.lastJiff
	t.lastJiff = jiff
	t.lastAt = now
	if elapsed <= 0 {
		return 0
	}
	// jiffies are in clock ticks (100/s on Linux)
	return float64(delta) / (elapsed * 100) * 100
}

func readCPUJiffies() (uint64, error) {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, err
	}
	// comm field may contain spaces; find closing ')' and split after it
	idx := strings.LastIndex(string(data), ")")
	if idx < 0 {
		return 0, fmt.Errorf("unexpected /proc/self/stat format")
	}
	fields := strings.Fields(string(data)[idx+1:])
	// fields[0]='state', fields[11]=utime, fields[12]=stime (0-indexed after ')')
	if len(fields) < 13 {
		return 0, fmt.Errorf("too few fields in /proc/self/stat")
	}
	utime, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return 0, err
	}
	stime, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return 0, err
	}
	return utime + stime, nil
}

func processMemRSS() int64 {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseInt(fields[1], 10, 64)
				if err == nil {
					return kb * 1024
				}
			}
		}
	}
	return 0
}

func systemMemInfo() (total, free int64) {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		return 0, 0
	}
	unit := int64(info.Unit)
	return int64(info.Totalram) * unit, int64(info.Freeram) * unit
}

func osName() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return runtime.GOOS + "/" + runtime.GOARCH
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			val := strings.TrimPrefix(line, "PRETTY_NAME=")
			val = strings.Trim(val, `"`)
			return val
		}
	}
	return runtime.GOOS + "/" + runtime.GOARCH
}
