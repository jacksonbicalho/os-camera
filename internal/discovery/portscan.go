package discovery

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	rtspPort    = 554
	dialTimeout = 500 * time.Millisecond
	scanWorkers = 50
)

// localSubnet returns the /24 subnet of the first non-loopback IPv4 interface.
func localSubnet() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}
			parts := strings.Split(ip.String(), ".")
			if len(parts) != 4 {
				continue
			}
			return fmt.Sprintf("%s.%s.%s.0/24", parts[0], parts[1], parts[2]), nil
		}
	}
	return "", fmt.Errorf("no suitable network interface found")
}

// scanPort554 scans all hosts in the given /24 subnet for open port 554.
func scanPort554(ctx context.Context, subnet string) []Result {
	ip, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		return nil
	}
	ip = ip.Mask(ipNet.Mask)

	type job struct{ ip string }
	jobs := make(chan job, 256)

	go func() {
		defer close(jobs)
		for cur := cloneIP(ip); ipNet.Contains(cur); inc(cur) {
			s := cur.String()
			// skip network address and broadcast
			if s == ip.String() {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case jobs <- job{s}:
			}
		}
	}()

	var (
		mu      sync.Mutex
		results []Result
		wg      sync.WaitGroup
	)

	for i := 0; i < scanWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				addr := fmt.Sprintf("%s:%d", j.ip, rtspPort)
				conn, err := net.DialTimeout("tcp", addr, dialTimeout)
				if err != nil {
					continue
				}
				conn.Close()
				mu.Lock()
				results = append(results, Result{IP: j.ip, Port: rtspPort, ONVIF: false})
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return results
}

func cloneIP(ip net.IP) net.IP {
	c := make(net.IP, len(ip))
	copy(c, ip)
	return c
}

func inc(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}
