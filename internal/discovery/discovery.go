package discovery

import (
	"context"
	"sync"
	"time"
)

const defaultTimeout = 10 * time.Second

// Discover runs ONVIF WS-Discovery and port-554 scan in parallel and returns
// deduplicated results. ONVIF results take precedence for the same IP.
func Discover(ctx context.Context) []Result {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	var (
		onvifResults []Result
		scanResults  []Result
		wg           sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		onvifResults = discoverONVIF(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		subnet, err := localSubnet()
		if err != nil {
			return
		}
		scanResults = scanPort554(ctx, subnet)
	}()

	wg.Wait()

	// Merge: ONVIF results first, then port-scan results for IPs not yet seen.
	seen := map[string]bool{}
	var combined []Result
	for _, r := range onvifResults {
		seen[r.IP] = true
		combined = append(combined, r)
	}
	for _, r := range scanResults {
		if seen[r.IP] {
			continue
		}
		seen[r.IP] = true
		combined = append(combined, r)
	}
	return combined
}
