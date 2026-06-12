package deviceinfo

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// digestCGI is a CGIClient that talks to a camera's /cgi-bin endpoints using
// HTTP Digest authentication (the scheme Dahua/Intelbras cameras require).
type digestCGI struct {
	host     string
	username string
	password string
	client   *http.Client
}

// newDigestCGI builds the production CGI client for a target.
func newDigestCGI(t Target) CGIClient {
	return &digestCGI{
		host:     t.Host,
		username: t.Username,
		password: t.Password,
		client:   &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *digestCGI) Get(ctx context.Context, query string) (string, error) {
	url := fmt.Sprintf("http://%s/cgi-bin/%s", c.host, query)

	// First request: expect a 401 carrying the digest challenge.
	resp, err := c.do(ctx, url, "")
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return string(body), nil
	}
	challenge := resp.Header.Get("WWW-Authenticate")
	resp.Body.Close()

	auth, err := c.authHeader(challenge, "GET", "/cgi-bin/"+query)
	if err != nil {
		return "", err
	}

	resp, err = c.do(ctx, url, auth)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cgi %s: status %d", query, resp.StatusCode)
	}
	return string(body), nil
}

func (c *digestCGI) do(ctx context.Context, url, auth string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	return c.client.Do(req)
}

// authHeader computes the Digest Authorization header for a challenge.
func (c *digestCGI) authHeader(challenge, method, uri string) (string, error) {
	if !strings.HasPrefix(challenge, "Digest ") {
		return "", fmt.Errorf("unexpected auth challenge: %q", challenge)
	}
	p := parseChallenge(challenge[len("Digest "):])
	realm, nonce, qop := p["realm"], p["nonce"], p["qop"]
	if realm == "" || nonce == "" {
		return "", fmt.Errorf("incomplete digest challenge")
	}

	ha1 := md5hex(c.username + ":" + realm + ":" + c.password)
	ha2 := md5hex(method + ":" + uri)

	var response, extra string
	if strings.Contains(qop, "auth") {
		cnonce := randHex(8)
		nc := "00000001"
		response = md5hex(strings.Join([]string{ha1, nonce, nc, cnonce, "auth", ha2}, ":"))
		extra = fmt.Sprintf(`, qop=auth, nc=%s, cnonce=%q`, nc, cnonce)
	} else {
		response = md5hex(ha1 + ":" + nonce + ":" + ha2)
	}

	header := fmt.Sprintf(`Digest username=%q, realm=%q, nonce=%q, uri=%q, response=%q`,
		c.username, realm, nonce, uri, response)
	if opaque := p["opaque"]; opaque != "" {
		header += fmt.Sprintf(`, opaque=%q`, opaque)
	}
	return header + extra, nil
}

// parseChallenge splits a comma-separated "key=value" digest challenge,
// stripping surrounding quotes from values.
func parseChallenge(s string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(s, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		out[strings.TrimSpace(kv[0])] = strings.Trim(strings.TrimSpace(kv[1]), `"`)
	}
	return out
}

func md5hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
