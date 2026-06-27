package discovery

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"

	"camera/internal/webcam"
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
	// Webcams locais (v4l2) — restream embutido. Não passam pelo dedup por IP
	// (todas em 127.0.0.1); distinguidas por Kind.
	combined = append(combined, webcamResults(webcam.Detected(), webcam.DefaultRTSPAddress)...)
	return combined
}

// webcamResults mapeia as webcams locais detectadas para Result{Kind:"webcam"}
// com a URL do restream embutido. Função pura (testável).
func webcamResults(devices []webcam.Device, addr string) []Result {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		host, portStr = addr, "8554"
	}
	port, _ := strconv.Atoi(portStr)
	out := make([]Result, 0, len(devices))
	for _, d := range devices {
		out = append(out, Result{
			Kind:     "webcam",
			Name:     d.Name,
			IP:       host,
			Port:     port,
			RTSPURLs: []string{webcam.RTSPURL(addr, d.RTSPName)},
		})
	}
	return out
}
