package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"camera/internal/db"
)

// S3Drive uploads objects to an S3-compatible bucket using AWS Signature V4.
type S3Drive struct {
	endpoint  string
	bucket    string
	region    string
	accessKey string
	secretKey string
	prefix    string
}

func NewS3Drive(dr db.Drive) *S3Drive {
	endpoint := dr.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://s3.%s.amazonaws.com", dr.Region)
	}
	// Normalise: no trailing slash.
	endpoint = strings.TrimRight(endpoint, "/")
	return &S3Drive{
		endpoint:  endpoint,
		bucket:    dr.Bucket,
		region:    dr.Region,
		accessKey: dr.AccessKey,
		secretKey: dr.SecretKey,
		prefix:    strings.TrimRight(dr.Prefix, "/"),
	}
}

func (s *S3Drive) Upload(ctx context.Context, key string, r io.Reader, size int64) error {
	if s.prefix != "" {
		key = s.prefix + "/" + key
	}

	body, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("s3: read body: %w", err)
	}

	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	encodedKey := awsEncodeKey(key)
	objectURL := fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, encodedKey)

	contentHash := hashHex(body)

	headers := map[string]string{
		"host":                 hostFromURL(s.endpoint),
		"x-amz-date":          amzDate,
		"x-amz-content-sha256": contentHash,
		"content-type":        "application/octet-stream",
	}

	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"

	canonicalHeaders := fmt.Sprintf(
		"content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		headers["content-type"],
		headers["host"],
		contentHash,
		amzDate,
	)

	canonicalRequest := strings.Join([]string{
		"PUT",
		"/" + s.bucket + "/" + encodedKey,
		"",
		canonicalHeaders,
		signedHeaders,
		contentHash,
	}, "\n")

	credentialScope := dateStamp + "/" + s.region + "/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + credentialScope + "\n" + hashHex([]byte(canonicalRequest))

	signingKey := hmacSHA256(
		hmacSHA256(
			hmacSHA256(
				hmacSHA256([]byte("AWS4"+s.secretKey), []byte(dateStamp)),
				[]byte(s.region),
			),
			[]byte("s3"),
		),
		[]byte("aws4_request"),
	)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKey, credentialScope, signedHeaders, signature,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("s3: build request: %w", err)
	}
	req.Header.Set("Host", headers["host"])
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", contentHash)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Authorization", authHeader)
	req.ContentLength = int64(len(body))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("s3: upload %s: %w", key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("s3: upload %s: status %d: %s", key, resp.StatusCode, strings.TrimSpace(string(errBody)))
	}
	return nil
}

func hostFromURL(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return endpoint
	}
	return u.Host
}

// awsEncodeKey percent-encodes an S3 object key for use in SigV4 canonical
// requests and request URLs. Each path segment (split on '/') is encoded
// independently so that '/' is preserved as a path separator. Only the AWS
// unreserved characters (A-Za-z0-9 _ - ~ .) are left unencoded; everything
// else — including '+' — is encoded as %XX.
func awsEncodeKey(key string) string {
	segments := strings.Split(key, "/")
	for i, seg := range segments {
		var buf strings.Builder
		for _, b := range []byte(seg) {
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') ||
				b == '_' || b == '-' || b == '~' || b == '.' {
				buf.WriteByte(b)
			} else {
				fmt.Fprintf(&buf, "%%%02X", b)
			}
		}
		segments[i] = buf.String()
	}
	return strings.Join(segments, "/")
}

func hashHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
