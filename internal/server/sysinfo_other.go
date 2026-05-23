//go:build !linux

package server

import (
	"runtime"
	"sync"
	"time"
)

type cpuTracker struct {
	mu       sync.Mutex
	lastJiff uint64
	lastAt   time.Time
}

func (t *cpuTracker) percent() float64 { return -1 }

func processMemRSS() int64              { return 0 }
func systemMemInfo() (int64, int64)     { return 0, 0 }
func osName() string                    { return runtime.GOOS + "/" + runtime.GOARCH }
